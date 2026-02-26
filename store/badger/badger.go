package badger

import (
    "fmt"

    "github.com/dgraph-io/badger/v4"
    "git.wyat.me/git-storage/object"
)

type BadgerStore struct {
    db *badger.DB
}

func New(path string) (*BadgerStore, error) {
    opts := badger.DefaultOptions(path).WithLogger(nil)
    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("open badger: %w", err)
    }
    return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Put(obj *object.Object) (string, error) {
    compressed, sha, err := object.Serialize(obj)
    if err != nil {
        return "", fmt.Errorf("serialize: %w", err)
    }

    err = s.db.Update(func(txn *badger.Txn) error {
        _, err := txn.Get([]byte(sha))
        if err == nil {
            return nil // already exists, nothing to do
        }
        if err != badger.ErrKeyNotFound {
            return err
        }
        return txn.Set([]byte(sha), compressed)
    })
    if err != nil {
        return "", fmt.Errorf("put: %w", err)
    }

    return sha, nil
}

func (s *BadgerStore) Get(sha string) (*object.Object, error) {
    var compressed []byte

    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte(sha))
        if err == badger.ErrKeyNotFound {
            return fmt.Errorf("object not found: %s", sha)
        }
        if err != nil {
            return err
        }
        compressed, err = item.ValueCopy(nil)
        return err
    })
    if err != nil {
        return nil, err
    }

    return object.Deserialize(compressed)
}

func (s *BadgerStore) Exists(sha string) (bool, error) {
    err := s.db.View(func(txn *badger.Txn) error {
        _, err := txn.Get([]byte(sha))
        return err
    })
    if err == badger.ErrKeyNotFound {
        return false, nil
    }
    if err != nil {
        return false, fmt.Errorf("exists: %w", err)
    }
    return true, nil
}

func (s *BadgerStore) Close() error {
    return s.db.Close()
}
