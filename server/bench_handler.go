package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	w.Header().Set("Content-Type", "application/json")

	run := bench.RunResult{Timestamp: time.Now()}

	// SQLite — temp file for benchmarks
	sqliteFile := filepath.Join(os.TempDir(), fmt.Sprintf("sqlite-bench-%d.db", time.Now().UnixNano()))
	defer os.Remove(sqliteFile)

	sqliteStore, err := sqlite.New(sqliteFile)
	if err != nil {
		http.Error(w, "failed to create sqlite store", http.StatusInternalServerError)
		return
	}
	defer sqliteStore.Close()
	run.Backends = append(run.Backends, bench.RunBackend("SQLite", sqliteStore))

	// BadgerDB — temp dir
	badgerDir, err := os.MkdirTemp("", "badger-bench-*")
	if err != nil {
		http.Error(w, "failed to create badger temp dir", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(badgerDir)

	badgerStore, err := badger.New(badgerDir)
	if err != nil {
		http.Error(w, "failed to create badger store", http.StatusInternalServerError)
		return
	}
	defer badgerStore.Close()
	run.Backends = append(run.Backends, bench.RunBackend("BadgerDB", badgerStore))

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = os.Getenv("ENDPOINT")
	}

	log.Printf("minio endpoint: %q", minioEndpoint)
	log.Printf("bucket: %q", os.Getenv("BUCKET"))

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
		if err == nil {
			run.Backends = append(run.Backends, bench.RunBackend("MinIO/S3", minioStore))
		}
	}

	history.mu.Lock()
	history.results = append(history.results, run)
	history.mu.Unlock()

	json.NewEncoder(w).Encode(run)
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
