package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.wyat.me/git-storage/bench"
	"git.wyat.me/git-storage/store/badger"
	ministore "git.wyat.me/git-storage/store/minio"
	"git.wyat.me/git-storage/store/sqlite"
)

type benchHistory struct {
	mu      sync.Mutex
	results []bench.RunResult
}

var history = &benchHistory{}

func (s *Server) handleBenchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	sendEvent := func(event string, data any) {
		b, err := json.Marshal(data)
		if err != nil {
			log.Printf("marshal event %s: %v", event, err)
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	run := bench.RunResult{Timestamp: time.Now()}

	// SQLite — temp file for benchmarks
	sqliteFile := filepath.Join(os.TempDir(), fmt.Sprintf("sqlite-bench-%d.db", time.Now().UnixNano()))
	defer os.Remove(sqliteFile)

	sqliteStore, err := sqlite.New(sqliteFile)
	if err != nil {
		sendEvent("error", map[string]string{"message": "failed to create sqlite store"})
		return
	}
	defer sqliteStore.Close()
	sendEvent("progress", map[string]string{"backend": "SQLite", "status": "running"})
	result := bench.RunBackend("SQLite", sqliteStore)
	run.Backends = append(run.Backends, result)
	sendEvent("backend", result)

	// BadgerDB — temp dir
	badgerDir, err := os.MkdirTemp("", "badger-bench-*")
	if err != nil {
		sendEvent("error", map[string]string{"message": "failed to create badger temp dir"})
		return
	}
	defer os.RemoveAll(badgerDir)

	badgerStore, err := badger.New(badgerDir)
	if err != nil {
		sendEvent("error", map[string]string{"message": "failed to create badger store"})
		return
	}
	defer badgerStore.Close()
	sendEvent("progress", map[string]string{"backend": "BadgerDB", "status": "running"})
	result = bench.RunBackend("BadgerDB", badgerStore)
	run.Backends = append(run.Backends, result)
	sendEvent("backend", result)

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = os.Getenv("ENDPOINT")
	}

	// Railway bucket endpoint includes https:// prefix, strip it
	minioEndpoint = strings.TrimPrefix(minioEndpoint, "https://")
	minioEndpoint = strings.TrimPrefix(minioEndpoint, "http://")

	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("ACCESS_KEY_ID")
	}
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("SECRET_ACCESS_KEY")
	}
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = os.Getenv("BUCKET")
	}

	if minioEndpoint != "" {
		minioStore, err := ministore.New(
			minioEndpoint,
			accessKey,
			secretKey,
			bucket,
			true, // Railway buckets use SSL
		)
		if err != nil {
			log.Printf("minio init failed (skipping): %v", err)
		} else {
			defer minioStore.Flush()
			sendEvent("progress", map[string]string{"backend": "MinIO/S3", "status": "running"})
			result = bench.RunBackend("MinIO/S3", minioStore)
			run.Backends = append(run.Backends, result)
			sendEvent("backend", result)
		}
	}

	history.mu.Lock()
	history.results = append(history.results, run)
	history.mu.Unlock()

	sendEvent("done", run)
}

func (s *Server) handleBenchHistory(w http.ResponseWriter, r *http.Request) {
	history.mu.Lock()
	defer history.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history.results)
}

func (s *Server) handleBenchUI(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("static", "bench.html"))
}
