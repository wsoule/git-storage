# git-storage

A working git remote server in Go with swappable storage backends. Built to empirically reproduce the storage tradeoffs that teams hit when building git infrastructure for AI coding platforms.

Clone and push real repositories to it (when running):

git clone http://localhost:8080/myrepo.git


## Why this exists

Teams building git infrastructure for AI coding platforms commonly reach for S3/R2 or Postgres as their object store. This project reproduces those choices as runnable backends and benchmarks them against a purpose-built key-value store.

The results are not subtle.

## Architecture

A single `ObjectStore` interface with three implementations:
```go
type ObjectStore interface {
    Put(obj *object.Object) (sha string, err error)
    Get(sha string) (*object.Object, error)
    Exists(sha string) (bool, error)
}
```

Git's Smart HTTP protocol is handled by delegating to `git http-backend` over CGI. All three backends sit behind the same interface — the HTTP layer never knows which one it's talking to.

### Object model

Git objects are stored in git's native format: `"<type> <size>\0<data>"`, SHA-1 hashed, zlib compressed. The SHA is computed on the uncompressed content, matching git's actual on-disk format exactly. This was verified against `git hash-object`.

### Backends

**SQLite** — the relational square peg. Git's object model is a content-addressed key-value store. Mapping it onto a SQL table works, but you pay for query planning, row overhead, and serialized writes on every operation. Under concurrent load, SQLite can only write through a single connection — `SQLITE_BUSY` errors are the alternative.

**BadgerDB** — the natural fit. A pure Go LSM-tree key-value store. SHA is the key, zlib-compressed object bytes are the value. No schema, no query planner, no impedance mismatch. Concurrent reads and writes are first-class.

**MinIO/S3** — the industry status quo. Every operation is an HTTP round trip. `Exists` checks — which git calls constantly during push to avoid resending objects — cost the same as a full object fetch. The per-request overhead dominates at small object sizes, which is most of git's workload.

## Benchmark results

Run live at `/bench`. These numbers were produced on a 2020 MacBook Pro (intel Pro), with MinIO running locally in Docker.

### Small objects (1KB) — git's most common case

| Operation      | SQLite    | BadgerDB   | MinIO     |
|----------------|-----------|------------|-----------|
| Put            | 4,408     | 6,245      | 807       |
| Get            | 33,020    | 148,481    | 820       |
| Exists         | 50,606    | 557,842    | 951       |
| Concurrent Put | 182       | 886        | 79        |

### Medium objects (100KB)

| Operation      | SQLite    | BadgerDB   | MinIO     |
|----------------|-----------|------------|-----------|
| Put            | 443       | 543        | 304       |
| Get            | 3,253     | 8,172      | 496       |
| Exists         | 53,469    | 62,803     | 922       |
| Concurrent Put | 52        | 215        | 15        |

### Large objects (1MB)

| Operation      | SQLite    | BadgerDB   | MinIO     |
|----------------|-----------|------------|-----------|
| Put            | 49        | 52         | 47        |
| Get            | 362       | 1,314      | 119       |
| Exists         | 54,102    | 594,703    | 932       |
| Concurrent Put | 6         | 22         | 4         |

### What the numbers say

**Exists is the most important operation nobody talks about.** During a push, git calls Exists for every object to avoid resending data the server already has. BadgerDB handles this at 557k ops/sec for small objects. MinIO handles it at 951 ops/sec — every check is an HTTP round trip regardless of object size. That's a 580x difference on the operation git calls most.

**MinIO's per-request overhead dominates small objects.** A 1KB Get on MinIO costs roughly the same as a 1MB Get — ~1ms either way. That's the HTTP round trip floor. For large objects MinIO is competitive on Put (47 vs 52 ops/sec), because S3-style stores are designed for large sequential writes. Git's workload is the opposite.

**SQLite's concurrent write story is honest but bad.** Without WAL mode and a connection limit of 1, concurrent writes produce `SQLITE_BUSY` errors. With those fixes, concurrent Put at 1MB is 6 ops/sec. BadgerDB manages 22. The gap widens as object size shrinks — at 1KB it's 182 vs 886.

**At large object sizes, all three backends converge on Put throughput.** The bottleneck becomes I/O, not the storage layer. This is the only case where the backend choice doesn't matter much.

## Running locally
```bash
# start the server (BadgerDB by default)
go run main.go

# with MinIO
docker run -d -p 9000:9000 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data

MINIO_ENDPOINT=localhost:9000 \
MINIO_ACCESS_KEY=minioadmin \
MINIO_SECRET_KEY=minioadmin \
go run main.go
```

Open `http://localhost:8080/bench` to run benchmarks and view results.

## TODO

### Protocol
- [ ] Implement native packfile parsing (currently delegating to `git http-backend`)
  - Parse packfile binary format directly in Go
  - Remove dependency on git being installed on the server

### Storage
- [ ] Implement a ref store interface alongside ObjectStore
  - `refs/heads/*`, `refs/tags/*` etc. need to be stored and queried
  - Currently managed by git http-backend on disk
- [ ] Back refs and objects entirely by BadgerDB (no local disk dependency)
  - Would remove need for Railway persistent volume
  - True distributed-systems storage backend story

### Hosting
- [ ] Replace Railway persistent volume with fully remote-backed storage
  - Prerequisite: ref store + native packfile implementation above
- [ ] Horizontal scaling
  - Single Railway instance + persistent volume can't scale out
  - Blocked on removing local disk dependency

### Benchmarks
- [ ] Write benchmark suite comparing all three backends
  - Put/Get throughput
  - Latency under distribution charts
  - Storage efficiency (SQLite overhead vs raw key-value)

  ---
  tiny
