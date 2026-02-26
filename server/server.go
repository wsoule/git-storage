package server

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

type Server struct {
	repoRoot string
}

func New(repoRoot string) (*Server, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0755); err != nil {
		return nil, fmt.Errorf("create repo root: %w", err)
	}
	return &Server{repoRoot: absRoot}, nil
}

func (s *Server) Handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/bench/run", s.handleBenchRun)
    mux.HandleFunc("/bench/history", s.handleBenchHistory)
    mux.HandleFunc("/bench", s.handleBenchUI)
    mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    mux.HandleFunc("/", s.handleRoot)
    return mux
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/" {
        http.ServeFile(w, r, filepath.Join("static", "index.html"))
        return
    }
    // everything else falls through to git
    if !strings.Contains(r.URL.Path, ".git") {
        http.NotFound(w, r)
        return
    }
    s.handleGit(w, r)
}

func (s *Server) handleGit(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	repoName := parts[0]
	repoPath := filepath.Join(s.repoRoot, repoName)

	if !isValidRepoName(repoName) {
    http.NotFound(w, r)
    return
}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		log.Printf("creating bare repo at %s", repoPath)
		if err := initBareRepo(repoPath); err != nil {
			log.Printf("init bare repo failed: %v", err)
			http.Error(w, "failed to init repo", http.StatusInternalServerError)
			return
		}
		log.Printf("bare repo created successfully")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		http.Error(w, "git not found", http.StatusInternalServerError)
		return
	}

	handler := &cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env: []string{
			"GIT_PROJECT_ROOT=" + s.repoRoot,
			"GIT_HTTP_EXPORT_ALL=1",
			},
		InheritEnv: []string{"PATH", "HOME", "USER"},
	}

	handler.ServeHTTP(w, r)
}

func isValidRepoName(name string) bool {
    if !strings.HasSuffix(name, ".git") {
        return false
    }
    // only allow alphanumeric, hyphens, underscores, dots
    for _, c := range strings.TrimSuffix(name, ".git") {
        if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '-' && c != '_' && c != '.' {
            return false
        }
    }
    return true
}

func initBareRepo(path string) error {
	cmd := exec.Command("git", "init", "--bare", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init --bare: %w\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", path, "config", "http.receivepack", "true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config http.receivepack: %w\n%s", err, out)
	}
	return nil
}
