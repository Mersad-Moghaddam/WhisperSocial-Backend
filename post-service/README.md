# Post Service - Wisper Creation & Distribution

The **Post Service** is responsible for creating wispers (posts) and publishing them to Redis Streams for asynchronous distribution to follower timelines.

## 🎯 Purpose

This service handles:
- Creating new wispers from authenticated users
- Persisting wispers to MySQL database
- Publishing wisper creation events to Redis Stream
- Validating wisper content (length, format)
- Providing wisper metadata (ID, timestamp, author)

## 🏗️ Architecture

### Technology Stack
- **Framework**: Go + Fiber (HTTP)
- **Database**: MySQL via GORM
- **Event Bus**: Redis Streams
- **Authentication**: JWT token validation

### Data Model

```go
type Post struct {
    ID        uint      `gorm:"primaryKey"`
    AuthorID  uint      `gorm:"not null;index"`
    Content   string    `gorm:"type:text;not null"`
    CreatedAt time.Time `gorm:"index"`
}
```

### Database Schema

```sql
CREATE TABLE posts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    author_id INT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_author_created (author_id, created_at DESC),
    INDEX idx_created (created_at DESC)
);
```

## 🔌 HTTP API

### Create Wisper

**Endpoint:** `POST /posts`

**Authentication:** Required (JWT Bearer token)

**Request Body:**
```json
{
  "content": "This is my first wisper!"
}
```

**Request Headers:**
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json
```

**Success Response (200 OK):**
```json
{
  "id": 123,
  "author_id": 42,
  "content": "This is my first wisper!",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**

- **401 Unauthorized**: Missing or invalid JWT token
```json
{
  "error": "unauthorized"
}
```

- **400 Bad Request**: Invalid content (empty or too long)
```json
{
  "error": "content is required"
}
```

- **500 Internal Server Error**: Database or Redis error
```json
{
  "error": "failed to create post"
}
```

### Health Check

**Endpoint:** `GET /health`

**Authentication:** Required (JWT Bearer token)

**Success Response (200 OK):**
```json
{
  "status": "healthy"
}
```

## 📡 Event Streaming

### Redis Stream Publication

When a wisper is successfully created, an event is published to Redis Stream for async fanout.

**Stream Name:** `post_created_stream`

**Event Fields:**
```
post_id: "123"
author_id: "42"
timestamp: "1642246800"  (Unix timestamp)
```

**Example XADD Command:**
```
XADD post_created_stream * post_id 123 author_id 42 timestamp 1642246800
```

This event is consumed by the **fanout-worker** service which distributes the wisper to all followers' timelines.

## 🔧 Configuration

### Environment Variables

Create a `.env` file in the service directory:

```env
# Database Configuration
DB_USER=root
DB_PASS=yourpassword
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=wisper

# Redis Configuration
REDIS_ADDR=localhost:6379

# JWT Secret (must match auth-service)
JWT_SECRET=your-secret-key-change-in-production

# Service Port
PORT=8081
```

### Required Services

- **MySQL** must be running and accessible
- **Redis** must be running for event streaming
- **auth-service** must be running for JWT validation

## 🚀 Running the Service

### Local Development

```bash
# Navigate to service directory
cd Backend/post-service

# Install dependencies (if needed)
go mod download

# Run the service
go run cmd/main.go
```

**Expected Output:**
```
[INFO] Connected to MySQL at 127.0.0.1:3306
[INFO] Auto-migrating Post model
[INFO] Connected to Redis at localhost:6379
[INFO] Post service listening on :8081
```

### Production Deployment

```bash
# Build binary
go build -o post-service cmd/main.go

# Run binary
./post-service
```

## 🧪 Testing

### Manual Testing with curl

**1. Register and login to get JWT token:**
```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@wisper.com","password":"test123"}' \
  | jq -r .token)
```

**2. Create a wisper:**
```bash
curl -X POST http://127.0.0.1:8081/posts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"content":"Hello Wisper world!"}' \
  | jq
```

**3. Verify in MySQL:**
```bash
mysql -u root -p wisper -e "SELECT * FROM posts ORDER BY created_at DESC LIMIT 1;"
```

**4. Verify Redis Stream:**
```bash
redis-cli XREVRANGE post_created_stream + - COUNT 1
```

### Integration Testing

```go
func TestCreatePost(t *testing.T) {
    // Setup test database and Redis
    
    // Create test user and get JWT
    
    // POST /posts with valid JWT
    
    // Assert post saved to MySQL
    
    // Assert event published to Redis Stream
}
```

## 🐛 Troubleshooting

### Common Issues

**Issue: "connection refused" to MySQL**
- Verify MySQL is running: `mysql -u root -h 127.0.0.1 -P 3306 -p`
- Check DB credentials in `.env`
- Ensure database exists: `CREATE DATABASE IF NOT EXISTS wisper;`

**Issue: "connection refused" to Redis**
- Verify Redis is running: `redis-cli ping`
- Check `REDIS_ADDR` in `.env`

**Issue: "unauthorized" when creating wisper**
- Verify JWT token is valid
- Check `JWT_SECRET` matches auth-service
- Ensure token hasn't expired

**Issue: Wisper created but not appearing on timelines**
- Check fanout-worker is running and consuming stream
- Verify Redis Stream has entries: `redis-cli XLEN post_created_stream`
- Check consumer group exists: `redis-cli XINFO GROUPS post_created_stream`

**Issue: Database errors**
- Check MySQL logs
- Verify table exists and schema is correct
- Ensure proper indexes: `SHOW INDEX FROM posts;`

## 📊 Monitoring

### Key Metrics

- **Requests per second**: POST /posts rate
- **Response time**: p50, p95, p99 latency
- **Error rate**: Failed wisper creations
- **Database latency**: MySQL INSERT time
- **Stream publish latency**: Redis XADD time

### Health Checks

```bash
# Service health
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8081/health

# Database connectivity
mysql -u root -p wisper -e "SELECT 1;"

# Redis connectivity
redis-cli ping

# Recent wispers
mysql -u root -p wisper -e "SELECT COUNT(*) FROM posts WHERE created_at > NOW() - INTERVAL 1 HOUR;"

# Stream backlog
redis-cli XLEN post_created_stream
```

### Logging

The service logs:
- Incoming requests
- Database operations
- Redis stream publications
- Errors and failures

**Example log output:**
```
[INFO] POST /posts - user_id: 42
[INFO] Post created: id=123
[INFO] Event published to stream: post_id=123
[ERROR] Failed to publish event: connection timeout
```

## 🔒 Security

### Input Validation

- **Content length**: Enforced (e.g., max 280 characters)
- **Content sanitization**: HTML/script tags stripped
- **Author ID**: Extracted from JWT, not from request body

### Authentication

- JWT token validated on every request
- Token must be signed with correct `JWT_SECRET`
- Expired tokens rejected

### Rate Limiting

Consider implementing:
- Per-user rate limits (e.g., 100 wispers/hour)
- Global rate limits to prevent abuse
- Use Redis for distributed rate limiting

## 📈 Scalability

### Horizontal Scaling

The post-service is **stateless** and can be scaled horizontally:

```bash
# Run multiple instances
PORT=8081 go run cmd/main.go  # Instance 1
PORT=8091 go run cmd/main.go  # Instance 2
PORT=8092 go run cmd/main.go  # Instance 3

# Load balancer (nginx, HAProxy) distributes traffic
```

### Database Optimization

- Use **connection pooling** (GORM default)
- Add indexes on frequently queried columns
- Consider **read replicas** for high read volumes
- Archive old wispers to cold storage

### Redis Stream Performance

- Redis Streams handle **millions of messages per second**
- Use **pipelining** for batch writes if needed
- Monitor stream length and consumer lag

## 🔄 Related Services

- **auth-service**: Validates JWT tokens
- **fanout-worker**: Consumes post_created events
- **timeline-service**: Reads distributed wispers
- **follow-service**: Provides follower lists to fanout-worker

## 📚 Additional Resources

- [Backend README](../README.md) - Overall architecture
- [RUNBOOK](../RUNBOOK.md) - Operational procedures
- [Redis Streams Documentation](https://redis.io/docs/data-types/streams/)
- [GORM Documentation](https://gorm.io/docs/)
- [Fiber Documentation](https://docs.gofiber.io/)

---

**Built with 🌤️ for Wisper**