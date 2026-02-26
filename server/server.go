package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Server struct {
	repoRoot string // Directory where bare repos live
}

func New(repoRoot string) (*Server, error) {
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		return nil, fmt.Errorf("create repo root: %w", err)
	}

	return &Server{repoRoot: repoRoot}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleGit)
	return mux
}

func (s *Server) handleGit(w http.ResponseWriter, r *http.Request) {
	// path is like /myrepo.git/info/refs or /myrepo.git/git-upload-pack
	// we need to split it into repo name and the rest
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	repoName := parts[0]
	pathInfo := "/" + parts[1]
	repoPath := filepath.Join(s.repoRoot, repoName)

	// create bare repo if it doesn't exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		if err := initBareRepo(repoPath); err != nil {
			http.Error(w, "failed to init repo", http.StatusInternalServerError)
			return
		}
	}

	cmd := exec.Command("git", "http-backend")
	cmd.Env = append(os.Environ(),
		"GIT_PROJECT_ROOT="+s.repoRoot,
		"GIT_HTTP_EXPORT_ALL=1",
		"PATH_INFO="+"/"+repoName+pathInfo,
		"QUERY_STRING="+r.URL.RawQuery,
		"REQUEST_METHOD="+r.Method,
		"CONTENT_TYPE="+r.Header.Get("Content-Type"),
		fmt.Sprintf("CONTENT_LENGTH=%d", r.ContentLength),
	)
	cmd.Stdin = r.Body

	out, err := cmd.Output()
	if err != nil {
		http.Error(w, "git http-backend failed", http.StatusInternalServerError)
		return
	}

	// git http-backend returns CGI format: header, blank line, body
	// we need to parse & forward that to the ResponseWriter
	headersEnd := strings.Index(string(out), "\r\n\r\n")
	if headersEnd == -1 {
		headersEnd = strings.Index(string(out), "\n\n")
		if headersEnd == -1 {
			http.Error(w, "invalid git http-backend response", http.StatusInternalServerError)
			return
		}
		headersEnd += 2
	} else {
		headersEnd += 4
	}

	headerLines := strings.Split(strings.TrimSpace(string(out[:headersEnd])), "\n")
	for _, line := range headerLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if strings.ToLower(key) == "status" {
			// e.g. "200 OK"
			fmt.Sscanf(value, "%d", new(int))
		} else {
			w.Header().Set(key, value)
		}
	}

	w.Write(out[headersEnd:])
}

func initBareRepo(repoPath string) error {
	cmd := exec.Command("git", "init", "--bare", repoPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init --bare: %w", err)
	}
	return nil
}
