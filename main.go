package main

import (
	"log"
	"net/http"
	"os"

	"git.wyat.me/git-storage/server"
)

func main() {
	repoRoot := os.Getenv("REPO_ROOT")
	if repoRoot == "" {
		repoRoot = "./repos"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv, err := server.New(repoRoot)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	log.Printf("listening on :%s, repos at %s", port, repoRoot)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
