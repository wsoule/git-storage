package object

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type ObjectType string

const (
	TypeBlob   ObjectType = "blob"
	TypeCommit ObjectType = "commit"
	TypeTree   ObjectType = "tree"
	TypeTag    ObjectType = "tag"
)

type Object struct {
	Type ObjectType
	Data []byte
}

func Serialize(obj *Object) (compressed []byte, sha string, err error) {
	header := fmt.Sprintf("%s %d\x00", obj.Type, len(obj.Data))
	content := append([]byte(header), obj.Data...)

	sum := sha1.Sum(content)
	sha = hex.EncodeToString(sum[:])

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err = w.Write(content); err != nil {
		return nil, "", fmt.Errorf("zlib write: %w", err)
	}
	if err = w.Close(); err != nil {
		return nil, "", fmt.Errorf("zlib close: %w", err)
	}

	return buf.Bytes(), sha, nil
}

func Deserialize(compressed []byte) (*Object, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("zlib new reader: %w", err)
	}
	defer r.Close()

	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("zlib read: %w", err)
	}
	return parse(content)
}

func parse(content []byte) (*Object, error) {
	nullIdx := bytes.IndexByte(content, 0)
	if nullIdx == -1 {
		return nil, fmt.Errorf("invalid object: no null byte")
	}

	header := string(content[:nullIdx])
	data := content[nullIdx+1:]

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid object header: %q", header)

	}

	size, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid size in header: %w", err)
	}
	if size != len(data) {
		return nil, fmt.Errorf("invalid data size: expected %d, got %d", size, len(data))
	}

	return &Object{
		Type: ObjectType(parts[0]),
		Data: data,
	}, nil
}
