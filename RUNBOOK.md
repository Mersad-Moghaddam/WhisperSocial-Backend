# 🌤️ Whisper Backend - Operations Runbook

This runbook covers day-2 operations: startup procedures, monitoring, debugging, recovery, and maintenance for the Wisper backend microservices.

---

## 📋 Table of Contents

- [Service Overview](#service-overview)
- [Startup Procedures](#startup-procedures)
- [Health Checks](#health-checks)
- [Operational Checks](#operational-checks)
- [Monitoring](#monitoring)
- [Common Issues & Troubleshooting](#common-issues--troubleshooting)
- [Recovery Procedures](#recovery-procedures)
- [Scaling Operations](#scaling-operations)
- [Security Operations](#security-operations)
- [API Quick Reference](#api-quick-reference)

---

## 🔧 Service Overview

### Processes and Ports

| Service | Port | Type | Dependencies |
|---------|------|------|--------------|
| **auth-service** | 8083 | HTTP | MySQL |
| **post-service** | 8081 | HTTP | MySQL, Redis |
| **follow-service** | 8085 | HTTP | MySQL |
| **timeline-service** | 8082 | HTTP | MySQL, Redis |
| **fanout-worker** | N/A | Background | MySQL, Redis |

### Infrastructure Dependencies

- **MySQL** (port 3306): Primary data store
- **Redis** (port 6379): Cache and stream broker

---

## 🚀 Startup Procedures

### Recommended Startup Order

**Critical Path:**
1. MySQL (infrastructure)
2. Redis (infrastructure)
3. auth-service (authentication)
4. follow-service (social graph)
5. post-service (wisper creation)
6. timeline-service (wisper delivery)
7. fanout-worker (async distribution)

### Step-by-Step Startup

#### 1. Start Infrastructure

**MySQL:**
```bash
# macOS
brew services start mysql@8.0

# Linux (systemd)
sudo systemctl start mysql

# Verify
mysql -u root -h 127.0.0.1 -P 3306 -p -e "SELECT 1;"
```

**Redis:**
```bash
# macOS
brew services start redis

# Linux (systemd)
sudo systemctl start redis

# Verify
redis-cli ping  # Should return: PONG
```

#### 2. Start Services

Each service loads `../.env` (or `.env` in its directory) and performs GORM auto-migration on startup.

**Auth Service (Terminal 1):**
```bash
cd Backend/auth-service
go run cmd/main.go

# Expected output:
# [INFO] Connected to MySQL
# [INFO] Auto-migrating User model
# [INFO] Auth service listening on :8083
```

**Follow Service (Terminal 2):**
```bash
cd Backend/follow-service
go run cmd/main.go

# Expected output:
# [INFO] Connected to MySQL
# [INFO] Auto-migrating Follower model
# [INFO] Follow service listening on :8085
```

**Post Service (Terminal 3):**
```bash
cd Backend/post-service
go run cmd/main.go

# Expected output:
# [INFO] Connected to MySQL
# [INFO] Connected to Redis
# [INFO] Auto-migrating Post model
# [INFO] Post service listening on :8081
```

**Timeline Service (Terminal 4):**
```bash
cd Backend/timeline-service
go run cmd/main.go

# Expected output:
# [INFO] Connected to MySQL
# [INFO] Connected to Redis
# [INFO] Timeline service listening on :8082
```

**Fanout Worker (Terminal 5):**
```bash
cd Backend/fanout-worker
go run cmd/main.go

# Expected output:
# [INFO] Connected to MySQL
# [INFO] Connected to Redis
# [INFO] Consumer group 'fanout_group' ready
# [INFO] Listening on stream 'post_created_stream'
```

### Production Startup (with systemd)

Example systemd service file (`/etc/systemd/system/wisper-auth.service`):

```ini
[Unit]
Description=Wisper Auth Service
After=mysql.service redis.service
Requires=mysql.service redis.service

[Service]
Type=simple
User=wisper
WorkingDirectory=/opt/wisper/auth-service
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin
EnvironmentFile=/opt/wisper/auth-service/.env
ExecStart=/usr/local/go/bin/go run cmd/main.go
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Start services:
```bash
sudo systemctl start wisper-auth
sudo systemctl start wisper-follow
sudo systemctl start wisper-post
sudo systemctl start wisper-timeline
sudo systemctl start wisper-fanout
```

---

## 🏥 Health Checks

### Manual Health Checks

**Auth Service:**
```bash
curl -i http://127.0.0.1:8083/health

# Expected: 200 OK
# {"status":"healthy"}
```

**Post Service:**
```bash
# Requires JWT token
curl -i -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8081/health

# Expected: 200 OK
```

**Follow Service:**
```bash
curl -i -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8085/health

# Expected: 200 OK
```

**Timeline Service:**
```bash
curl -i -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8082/health

# Expected: 200 OK
```

**Fanout Worker:**
```bash
# No HTTP endpoint - check logs and Redis
redis-cli XINFO GROUPS post_created_stream

# Should show consumer group 'fanout_group' with active consumers
```

### Automated Health Monitoring

Create a monitoring script (`health_check.sh`):

```bash
#!/bin/bash

services=(
  "auth:8083:/health"
  "post:8081:/health"
  "follow:8085:/health"
  "timeline:8082:/health"
)

TOKEN="your-test-token-here"

for svc in "${services[@]}"; do
  IFS=':' read -r name port path <<< "$svc"
  
  if [ "$name" == "auth" ]; then
    status=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${port}${path}")
  else
    status=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "http://127.0.0.1:${port}${path}")
  fi
  
  if [ "$status" == "200" ]; then
    echo "✅ $name-service: healthy"
  else
    echo "❌ $name-service: unhealthy (HTTP $status)"
  fi
done
```

### End-to-End Health Check

Test the complete flow:
```bash
# 1. Register user
curl -X POST http://127.0.0.1:8083/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@wisper.com","password":"test123"}'

# 2. Login
TOKEN=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@wisper.com","password":"test123"}' \
  | jq -r .token)

# 3. Create wisper
curl -X POST http://127.0.0.1:8081/posts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Health check wisper"}'

# 4. Verify timeline
curl -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8082/timeline?limit=1"

# 5. Check Redis stream
redis-cli XLEN post_created_stream
```

---

## 🔍 Operational Checks

### Database Health

**MySQL:**
```bash
# Connection test
mysql -u root -h 127.0.0.1 -P 3306 -p -e "SELECT 1;"

# Check tables exist
mysql -u root -p wisper -e "SHOW TABLES;"

# Expected tables:
# - users
# - posts
# - followers

# Check record counts
mysql -u root -p wisper -e "
  SELECT 'users' AS table_name, COUNT(*) AS count FROM users
  UNION ALL
  SELECT 'posts', COUNT(*) FROM posts
  UNION ALL
  SELECT 'followers', COUNT(*) FROM followers;
"

# Check recent wispers
mysql -u root -p wisper -e "
  SELECT id, author_id, LEFT(content, 50) AS content, created_at 
  FROM posts 
  ORDER BY created_at DESC 
  LIMIT 5;
"

# Check active connections
mysql -u root -p -e "SHOW PROCESSLIST;"
```

### Redis Health

**Connection and Keys:**
```bash
# Connection test
redis-cli ping  # Should return: PONG

# Check Redis info
redis-cli INFO server
redis-cli INFO memory
redis-cli INFO stats

# List timeline keys
redis-cli KEYS 'timeline:*'

# Check specific user's timeline
redis-cli ZREVRANGE timeline:1 0 9 WITHSCORES

# Count timelines
redis-cli EVAL "return #redis.call('keys', 'timeline:*')" 0
```

**Stream Health:**
```bash
# Stream info
redis-cli XINFO STREAM post_created_stream

# Consumer groups
redis-cli XINFO GROUPS post_created_stream

# Pending messages
redis-cli XPENDING post_created_stream fanout_group

# Stream length
redis-cli XLEN post_created_stream

# View last 5 stream entries
redis-cli XREVRANGE post_created_stream + - COUNT 5

# Consumer info (detailed)
redis-cli XINFO CONSUMERS post_created_stream fanout_group
```

### Follow Relationships

```bash
# Get follower count per user
mysql -u root -p wisper -e "
  SELECT followee_id AS user_id, COUNT(*) AS followers 
  FROM followers 
  GROUP BY followee_id 
  ORDER BY followers DESC 
  LIMIT 10;
"

# Get following count per user
mysql -u root -p wisper -e "
  SELECT follower_id AS user_id, COUNT(*) AS following 
  FROM followers 
  GROUP BY follower_id 
  ORDER BY following DESC 
  LIMIT 10;
"

# Check mutual follows
mysql -u root -p wisper -e "
  SELECT a.follower_id, a.followee_id 
  FROM followers a
  INNER JOIN followers b 
    ON a.follower_id = b.followee_id 
    AND a.followee_id = b.follower_id
  LIMIT 10;
"
```

### Fanout Worker Status

```bash
# Check worker is consuming
redis-cli XINFO CONSUMERS post_created_stream fanout_group

# Check lag (pending messages)
redis-cli XPENDING post_created_stream fanout_group - + 10

# If lag is high, check worker logs for errors
```

---

## 📊 Monitoring

### Key Metrics to Monitor

#### Application Metrics

- **Request Rate**: Requests per second per service
- **Response Time**: p50, p95, p99 latency
- **Error Rate**: 4xx and 5xx responses
- **Timeline Reads**: Queries to timeline-service
- **Wisper Creates**: Posts to post-service
- **Follow Actions**: Follow/unfollow rate

#### Infrastructure Metrics

**MySQL:**
- Connection pool usage
- Query execution time
- Slow query log
- Table sizes
- Replication lag (if using replicas)

**Redis:**
- Memory usage
- Hit/miss ratio
- Connected clients
- Commands per second
- Stream consumer lag

#### Stream Processing

- **Stream Length**: `XLEN post_created_stream`
- **Pending Messages**: Consumer group lag
- **Processing Rate**: Messages processed per second
- **Fanout Time**: Time from post creation to timeline appearance

### Prometheus Metrics (if configured)

Example metrics to expose:

```
# Application
wisper_requests_total{service="post",method="POST",path="/posts"}
wisper_request_duration_seconds{service="post",quantile="0.95"}
wisper_errors_total{service="post",code="500"}

# Stream Processing
wisper_stream_length{stream="post_created_stream"}
wisper_consumer_lag{group="fanout_group"}
wisper_fanout_duration_seconds

# Social Graph
wisper_users_total
wisper_posts_total
wisper_follows_total
```

---

## 🐛 Common Issues & Troubleshooting

### Issue: Services Won't Start

**Symptoms:**
- Service exits immediately
- "connection refused" errors
- "bind: address already in use"

**Diagnosis:**
```bash
# Check if port is in use
lsof -i :8083  # Check auth-service port
lsof -i :8081  # Check post-service port

# Check MySQL connection
mysql -u root -h 127.0.0.1 -P 3306 -p -e "SELECT 1;"

# Check Redis connection
redis-cli ping

# Check environment variables
cd auth-service
cat .env
```

**Solutions:**
1. **Port conflict**: Stop conflicting process or change port in `.env`
2. **DB connection**: Verify credentials in `.env`, ensure MySQL is running
3. **Redis connection**: Verify `REDIS_ADDR` in `.env`, ensure Redis is running
4. **Missing .env**: Create `.env` file with required variables

### Issue: Timeline Not Updating

**Symptoms:**
- User creates wisper but followers don't see it
- Timeline shows old wispers only

**Diagnosis:**
```bash
# 1. Check fanout worker is running
ps aux | grep fanout-worker

# 2. Check Redis stream has entries
redis-cli XLEN post_created_stream

# 3. Check consumer group exists
redis-cli XINFO GROUPS post_created_stream

# 4. Check for pending messages (lag)
redis-cli XPENDING post_created_stream fanout_group

# 5. Check worker logs for errors
tail -f fanout-worker.log

# 6. Verify follower relationships exist
mysql -u root -p wisper -e "SELECT * FROM followers WHERE followee_id = 1;"

# 7. Check if timeline key exists for follower
redis-cli EXISTS timeline:2
```

**Solutions:**

**Fanout worker not running:**
```bash
cd Backend/fanout-worker
go run cmd/main.go
```

**Consumer group doesn't exist:**
```bash
redis-cli XGROUP CREATE post_created_stream fanout_group 0 MKSTREAM
```

**High lag (pending messages):**
```bash
# Scale workers - run additional instances
cd Backend/fanout-worker
go run cmd/main.go  # In new terminal

# Or reset consumer group (DANGER: replays all messages)
redis-cli XGROUP DESTROY post_created_stream fanout_group
redis-cli XGROUP CREATE post_created_stream fanout_group $
```

**No follower relationships:**
- Verify users have followed each other via follow-service API

### Issue: Authentication Failures

**Symptoms:**
- 401 Unauthorized responses
- "invalid token" errors
- Token validation fails

**Diagnosis:**
```bash
# Check JWT_SECRET is consistent across services
cd Backend/auth-service && grep JWT_SECRET .env
cd Backend/post-service && grep JWT_SECRET .env
cd Backend/follow-service && grep JWT_SECRET .env
cd Backend/timeline-service && grep JWT_SECRET .env

# Test token generation
TOKEN=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@wisper.com","password":"test123"}' \
  | jq -r .token)

echo $TOKEN

# Decode token (check expiration)
echo $TOKEN | cut -d'.' -f2 | base64 -d 2>/dev/null | jq
```

**Solutions:**
1. **Inconsistent JWT_SECRET**: Update `.env` files to use same secret, restart services
2. **Expired token**: Generate new token via login
3. **Malformed header**: Ensure format is `Authorization: Bearer <token>`

### Issue: Redis Memory Issues

**Symptoms:**
- Redis running out of memory
- Timeline queries slow
- Redis returns OOM errors

**Diagnosis:**
```bash
# Check memory usage
redis-cli INFO memory

# Check key count
redis-cli DBSIZE

# Find large keys
redis-cli --bigkeys

# Check timeline key sizes
redis-cli ZCARD timeline:1
redis-cli ZCARD timeline:2
```

**Solutions:**

**Set maxmemory and eviction policy:**
```bash
# In redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru

# Or via CLI
redis-cli CONFIG SET maxmemory 2gb
redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

**Trim old timeline entries:**
```bash
# Keep only last 1000 wispers per timeline
for key in $(redis-cli KEYS 'timeline:*'); do
  redis-cli ZREMRANGEBYRANK $key 0 -1001
done
```

**Archive old wispers:**
```bash
# Move wispers older than 30 days to cold storage
# (Implement application-level logic)
```

### Issue: MySQL Performance Problems

**Symptoms:**
- Slow timeline hydration
- High CPU on MySQL
- Query timeouts

**Diagnosis:**
```bash
# Check slow queries
mysql -u root -p -e "
  SELECT * FROM mysql.slow_log 
  ORDER BY start_time DESC 
  LIMIT 10;
"

# Check current queries
mysql -u root -p -e "SHOW FULL PROCESSLIST;"

# Check table sizes
mysql -u root -p wisper -e "
  SELECT 
    table_name,
    ROUND(((data_length + index_length) / 1024 / 1024), 2) AS size_mb
  FROM information_schema.TABLES
  WHERE table_schema = 'wisper'
  ORDER BY size_mb DESC;
"

# Check indexes
mysql -u root -p wisper -e "SHOW INDEX FROM posts;"
mysql -u root -p wisper -e "SHOW INDEX FROM followers;"
```

**Solutions:**

**Add indexes:**
```sql
-- Ensure these indexes exist
CREATE INDEX idx_posts_author_created ON posts(author_id, created_at DESC);
CREATE INDEX idx_posts_created ON posts(created_at DESC);
CREATE INDEX idx_followers_followee ON followers(followee_id);
CREATE INDEX idx_followers_follower ON followers(follower_id);
```

**Optimize queries:**
```sql
-- Use EXPLAIN to analyze queries
EXPLAIN SELECT * FROM posts WHERE author_id = 1 ORDER BY created_at DESC LIMIT 20;
```

**Enable query cache (MySQL 5.7):**
```sql
SET GLOBAL query_cache_size = 67108864; -- 64MB
SET GLOBAL query_cache_type = ON;
```

---

## 🔄 Recovery Procedures

### Rebuild User Timeline

If a user's timeline is corrupted or missing:

```bash
# Manual timeline rebuild script
cd Backend
cat > rebuild_timeline.sh << 'EOF'
#!/bin/bash

USER_ID=$1
if [ -z "$USER_ID" ]; then
  echo "Usage: $0 <user_id>"
  exit 1
fi

echo "Rebuilding timeline for user $USER_ID..."

# Get users that $USER_ID follows
FOLLOWING=$(mysql -u root -p -N wisper -e "
  SELECT followee_id FROM followers WHERE follower_id = $USER_ID;
" | tr '\n' ',')

FOLLOWING=${FOLLOWING%,}  # Remove trailing comma

if [ -z "$FOLLOWING" ]; then
  echo "User $USER_ID doesn't follow anyone."
  exit 0
fi

# Clear existing timeline
redis-cli DEL "timeline:$USER_ID"

# Get wispers from followed users and add to timeline
mysql -u root -p -N wisper -e "
  SELECT id, UNIX_TIMESTAMP(created_at) 
  FROM posts 
  WHERE author_id IN ($FOLLOWING)
  ORDER BY created_at DESC 
  LIMIT 1000;
" | while read post_id timestamp; do
  redis-cli ZADD "timeline:$USER_ID" "$timestamp" "$post_id"
done

echo "Timeline rebuilt. Total wispers: $(redis-cli ZCARD timeline:$USER_ID)"
EOF

chmod +x rebuild_timeline.sh

# Run it
./rebuild_timeline.sh 42
```

### Reset Redis Stream

If stream is corrupted or stuck:

```bash
# Delete and recreate stream
redis-cli DEL post_created_stream
redis-cli XGROUP CREATE post_created_stream fanout_group $ MKSTREAM

# Restart fanout worker
pkill -f fanout-worker
cd Backend/fanout-worker
go run cmd/main.go
```

### Database Backup and Restore

**Backup:**
```bash
# Full backup
mysqldump -u root -p wisper > wisper_backup_$(date +%Y%m%d_%H%M%S).sql

# Backup Redis
redis-cli SAVE
cp /var/lib/redis/dump.rdb ~/backups/redis_backup_$(date +%Y%m%d).rdb
```

**Restore:**
```bash
# Restore MySQL
mysql -u root -p wisper < wisper_backup_20240115_120000.sql

# Restore Redis
redis-cli SHUTDOWN
cp ~/backups/redis_backup_20240115.rdb /var/lib/redis/dump.rdb
redis-server
```

---

## 📈 Scaling Operations

### Horizontal Scaling

**Scale HTTP Services:**
```bash
# Run multiple instances behind a load balancer
# Example: Scale timeline-service

# Instance 1
PORT=8082 go run cmd/main.go

# Instance 2 (different port)
PORT=8092 go run cmd/main.go

# Configure load balancer (nginx example)
upstream timeline_backend {
  server 127.0.0.1:8082;
  server 127.0.0.1:8092;
}
```

**Scale Fanout Workers:**
```bash
# Run multiple workers in same consumer group
# They automatically distribute work

# Worker 1
CONSUMER_NAME=worker-1 go run cmd/main.go

# Worker 2
CONSUMER_NAME=worker-2 go run cmd/main.go

# Worker 3
CONSUMER_NAME=worker-3 go run cmd/main.go
```

### Database Scaling

**MySQL Read Replicas:**
- Configure read replicas for timeline hydration queries
- Route writes to primary, reads to replicas
- Monitor replication lag

**Redis Cluster:**
- Set up Redis cluster for horizontal scaling
- Shard timeline keys across nodes
- Use Redis Sentinel for high availability

---

## 🔒 Security Operations

### Rotate JWT Secret

```bash
# 1. Generate new secret
NEW_SECRET=$(openssl rand -hex 32)

# 2. Update all service .env files
for svc in auth-service post-service follow-service timeline-service; do
  echo "JWT_SECRET=$NEW_SECRET" >> Backend/$svc/.env
done

# 3. Restart all services
# 4. All existing tokens will be invalidated
# 5. Users need to log in again
```

### Monitor Failed Login Attempts

```bash
# Check auth-service logs for failed logins
grep "login failed" Backend/auth-service/logs/*.log | wc -l

# Implement rate limiting (application-level or nginx)
```

### Audit Log

Implement audit logging for sensitive operations:
- User registration
- Follow/unfollow actions
- Wisper creation
- Account changes

---

## 🛠️ API Quick Reference

### Auth Service (:8083)

**Register:**
```bash
curl -X POST http://127.0.0.1:8083/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'
```

**Login:**
```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}' \
  | jq -r .token)
```

### Post Service (:8081)

**Create Wisper:**
```bash
curl -X POST http://127.0.0.1:8081/posts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"content":"Hello Wisper!"}'
```

### Follow Service (:8085)

**Follow:**
```bash
curl -X POST http://127.0.0.1:8085/follow \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"user_id": 42}'
```

**Unfollow:**
```bash
curl -X POST http://127.0.0.1:8085/unfollow \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"user_id": 42}'
```

**Get Followers:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8085/followers/42
```

**Get Following:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8085/following/42
```

**Check Following Status:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8085/is-following/42
```

**Get Stats:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8085/stats/42
```

### Timeline Service (:8082)

**Get Timeline:**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8082/timeline?limit=20&cursor=0"
```

### Redis Checks

**Check Stream:**
```bash
redis-cli XINFO STREAM post_created_stream
redis-cli XLEN post_created_stream
redis-cli XPENDING post_created_stream fanout_group
```

**Check Timeline:**
```bash
redis-cli ZREVRANGE timeline:1 0 9 WITHSCORES
redis-cli ZCARD timeline:1
```

---

## 📞 Escalation and Support

### Log Locations

```
Backend/
├── auth-service/logs/
├── post-service/logs/
├── follow-service/logs/
├── timeline-service/logs/
└── fanout-worker/logs/
```

### Critical Alerts

Set up alerts for:
- Service down (health check fails)
- High error rate (>5% 5xx responses)
- Database connection failures
- Redis memory >90%
- Fanout lag >10 seconds
- MySQL replication lag >5 seconds

---

**Built with 🌤️ for Wisper Operations**