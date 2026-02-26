package minio

import (
    "bytes"
    "context"
    "fmt"
    "io"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    "git.wyat.me/git-storage/object"
)

type MinioStore struct {
    client *minio.Client
    bucket string
}

func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioStore, error) {
    client, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("create minio client: %w", err)
    }

    ctx := context.Background()
    exists, err := client.BucketExists(ctx, bucket)
    if err != nil {
        return nil, fmt.Errorf("check bucket: %w", err)
    }
    if !exists {
        if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
            return nil, fmt.Errorf("create bucket: %w", err)
        }
    }

    return &MinioStore{client: client, bucket: bucket}, nil
}

func (s *MinioStore) Put(obj *object.Object) (string, error) {
    compressed, sha, err := object.Serialize(obj)
    if err != nil {
        return "", fmt.Errorf("serialize: %w", err)
    }

    exists, err := s.Exists(sha)
    if err != nil {
        return "", err
    }
    if exists {
        return sha, nil
    }

    _, err = s.client.PutObject(
        context.Background(),
        s.bucket,
        sha,
        bytes.NewReader(compressed),
        int64(len(compressed)),
        minio.PutObjectOptions{ContentType: "application/octet-stream"},
    )
    if err != nil {
        return "", fmt.Errorf("put object: %w", err)
    }

    return sha, nil
}

func (s *MinioStore) Get(sha string) (*object.Object, error) {
    obj, err := s.client.GetObject(
        context.Background(),
        s.bucket,
        sha,
        minio.GetObjectOptions{},
    )
    if err != nil {
        return nil, fmt.Errorf("get object: %w", err)
    }
    defer obj.Close()

    compressed, err := io.ReadAll(obj)
    if err != nil {
        return nil, fmt.Errorf("read object: %w", err)
    }

    return object.Deserialize(compressed)
}

func (s *MinioStore) Exists(sha string) (bool, error) {
    _, err := s.client.StatObject(
        context.Background(),
        s.bucket,
        sha,
        minio.StatObjectOptions{},
    )
    if err != nil {
        errResp := minio.ToErrorResponse(err)
        if errResp.Code == "NoSuchKey" {
            return false, nil
        }
        return false, fmt.Errorf("stat object: %w", err)
    }
    return true, nil
}

// Flush removes all objects from the bucket. Used after benchmarks to avoid
// leaving test data in the bucket.
func (s *MinioStore) Flush() error {
    ctx := context.Background()

    objectsCh := make(chan minio.ObjectInfo)
    go func() {
        defer close(objectsCh)
        for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{}) {
            if obj.Err != nil {
                return
            }
            objectsCh <- obj
        }
    }()

    for result := range s.client.RemoveObjects(ctx, s.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
        if result.Err != nil {
            return fmt.Errorf("remove object %s: %w", result.ObjectName, result.Err)
        }
    }

    return nil
}

