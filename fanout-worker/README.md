### fanout_worker

Consumes `post_created_stream` and fans out posts to follower timelines. Also exposes follow/unfollow HTTP API.

#### Tech
- Go, Fiber (HTTP for follow/unfollow)
- Redis Streams consumer group (`fanout_group`)
- MySQL via GORM for follow relations

#### Environment
- `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_NAME`
- `REDIS_ADDR`

Loads `../.env` on startup.

#### Run
```bash
go run cmd/main.go
```
Starts HTTP on `:8084` and a background stream consumer.

#### HTTP API
- POST `/follow` `{ user_id, follower_id }`
- POST `/unfollow` `{ user_id, follower_id }`

#### Streams and Keys
- Consumes stream: `post_created_stream` using group `fanout_group` and consumer `fanout_worker`.
- Writes to Redis ZSET key: `timeline:{userID}` with member=postID and score=timestamp.
