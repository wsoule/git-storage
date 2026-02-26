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
  - Latency under concurrent load
  - Storage efficiency (SQLite overhead vs raw key-value)
  - Write up results in readme
