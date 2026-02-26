package sqlite

import (
	"testing"

	"git.wyat.me/git-storage/object"
)

func TestPutAndGet(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	obj := &object.Object{
		Type: object.TypeBlob,
		Data: []byte("hello\n"),
	}

	sha, err := store.Put(obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	const expectedSHA = "ce013625030ba8dba906f756967f9e9ca394464a"
	if sha != expectedSHA {
		t.Fatalf("Put returned %s, expected %s", sha, expectedSHA)
	}

	got, err := store.Get(sha)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Type != obj.Type {
		t.Fatalf("Get returned %v, expected %v", got.Type, obj.Type)
	}
	if string(got.Data) != string(obj.Data) {
		t.Errorf("Data mismatch: got %q, want %q", got.Data, obj.Data)
	}
}

func TestExists(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	obj := &object.Object{
		Type: object.TypeBlob,
		Data: []byte("hello\n"),
	}

	sha, err := store.Put(obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	exists, err := store.Exists(sha)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	
	if !exists {
		t.Error("expected object to exist after Put")
	}

	exists, err = store.Exists("0000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if exists {
		t.Error("expected fake SHA to not exist")
	}
}

func TestDuplicatePut(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	obj := &object.Object{
		Type: object.TypeBlob,
		Data: []byte("hello\n"),
	}

	sha1, err := store.Put(obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	sha2, err := store.Put(obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if sha1 != sha2 {
		t.Errorf("Put returned %s and %s, expected them to be the same", sha1, sha2)
	}
}
