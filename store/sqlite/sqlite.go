package sqlite

import (
	"database/sql"
	"fmt"

	"git.wyat.me/git-storage/object"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func New(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS objects (
			sha TEXT PRIMARY KEY,
			data BLOB
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &SQLiteStore{
		db: db,
	}, nil
}

func (s *SQLiteStore) Put(obj *object.Object) (sha string, err error) {
	compressed, sha, err := object.Serialize(obj)
	if err != nil {
		return "", fmt.Errorf("serialize: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO objects (sha, data) VALUES (?, ?)`,
		sha, compressed,
	)
	if err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	return sha, nil
}

func (s *SQLiteStore) Get(sha string) (*object.Object, error) {
	var compressed []byte
	err := s.db.QueryRow(`SELECT data FROM objects WHERE sha = ?`, sha).Scan(&compressed)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("object not found %s", sha)
	}
	if err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}

	return object.Deserialize(compressed)
}

func (s *SQLiteStore) Exists(sha string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM objects WHERE sha = ?`, sha).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("exists query: %w", err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
