package object

import (
	"testing"
)

func TestSerializeDeserializeRoundtrip (t *testing.T) {
	obj := &Object{
		Type: TypeBlob,
		Data: []byte("hello\n"),
	}

	compressed, sha, err := Serialize(obj)
	if err != nil {
		t.Fatal(err)
	}

	// git hash-object for hello is this sha, created using the following command:
	// echo "hello" | git hash-object --stdin
	const expectedSHA = "ce013625030ba8dba906f756967f9e9ca394464a"
	if sha != expectedSHA {
		t.Errorf("SHA mismatch: got %s, want %s", sha, expectedSHA)
	}

	got, err := Deserialize(compressed)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if got.Type != obj.Type {
		t.Errorf("Type mismatch: got %s, want %s", got.Type, obj.Type)
	}
	if string(got.Data) != string(obj.Data) {
		t.Errorf("Data mismatch: got %q, want %q", got.Data, obj.Data)
	}
}

