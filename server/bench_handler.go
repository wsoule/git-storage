package server

import (
	"encoding/json"
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

	// SQLite — in-memory for benchmarks
	sqliteStore, err := sqlite.New(":memory:")
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

	// MinIO — optional, skip if not configured
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint != "" {
		minioStore, err := ministore.New(
			minioEndpoint,
			os.Getenv("MINIO_ACCESS_KEY"),
			os.Getenv("MINIO_SECRET_KEY"),
			"bench-git-objects",
			false,
		)
		if err == nil {
			run.Backends = append(run.Backends, bench.RunBackend("MinIO", minioStore))
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
