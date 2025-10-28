### timeline_service

Reads user timelines from Redis and hydrates posts from MySQL.

#### Tech
- Go, Fiber
- Redis ZSET timelines via `shared/config/redis.go`
- MySQL via GORM for post hydration

#### Environment
- `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_NAME`
- `REDIS_ADDR`

Loads `../.env` on startup.

#### Run
```bash
go run cmd/main.go
```
Listens on `:8082`.

#### HTTP API
- GET `/timeline?userID={id}&cursor={ts?}&limit={n?}`
  - Returns: `{ posts: Post[], nextCursor: number }`
  - Pages by timestamp descending; `nextCursor` is the score of the last item.

#### Data Model
- Redis ZSET key: `timeline:{userID}` with members=postID, score=timestamp.
- Hydration via SQL `SELECT * FROM posts WHERE id IN (?)`.
