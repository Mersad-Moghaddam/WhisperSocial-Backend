# Wisper Backend - Microservices Architecture

A scalable social media backend system built with microservices architecture in Go. Powers the **Wisper** platform with real-time wisper distribution using event-driven design with Redis Streams for asynchronous processing.

## 🏗️ Architecture Overview

- **Storage**: MySQL via GORM for users, wispers, and follow relations
- **Cache/Stream**: Redis for sorted-set timelines and Redis Streams for eventing
- **Transport**: HTTP APIs using Fiber; background worker consumes streams
- **Design**: Event-driven microservices with clean architecture patterns

## 🔧 Services and Ports

| Service | Port | Description |
|---------|------|-------------|
| **auth-service** | 8083 | User registration and JWT-based authentication |
| **post-service** | 8081 | Create wispers and publish events to Redis Streams |
| **follow-service** | 8085 | Manage follow/unfollow relationships and social graph |
| **timeline-service** | 8082 | Read user timelines from Redis and hydrate wispers from MySQL |
| **fanout-worker** | N/A | Background consumer - processes wisper events and distributes to follower timelines |

## 📊 Data Flow

**Complete Wisper Creation → Timeline Distribution Flow:**

1. **Client creates a wisper** → `POST /posts` to `post-service` with JWT token
2. **Wisper persisted** → `post-service` saves wisper to MySQL `posts` table
3. **Event published** → `post-service` emits `post_created` event to Redis Stream `post_created_stream`
4. **Fanout worker consumes** → `fanout-worker` reads stream in consumer group `fanout_group`
5. **Follower query** → Worker queries `follow-service` data (or directly queries MySQL `followers` table)
6. **Timeline distribution** → Worker pushes wisper ID to each follower's Redis ZSET `timeline:{userID}` with score=timestamp
7. **Client fetches timeline** → `GET /timeline` from `timeline-service` with JWT
8. **Timeline served** → Service reads post IDs from Redis ZSET, hydrates full wisper data from MySQL
9. **Response returned** → Client receives paginated list of wispers with metadata

### Visual Flow Diagram

```
┌─────────┐
│ Client  │
└────┬────┘
     │ POST /posts (JWT)
     ▼
┌────────────────┐      ┌───────┐
│ post-service   │─────▶│ MySQL │ (save wisper)
│    :8081       │      └───────┘
└────┬───────────┘
     │ publish event
     ▼
┌────────────────────┐
│  Redis Stream      │
│ post_created_stream│
└────┬───────────────┘
     │ consume
     ▼
┌────────────────┐      ┌──────────────┐
│ fanout-worker  │─────▶│ MySQL        │
│  (background)  │      │ (get followers)
└────┬───────────┘      └──────────────┘
     │ ZADD timeline:{followerID}
     ▼
┌────────────────┐
│  Redis ZSETs   │
│ timeline:*     │
└────┬───────────┘
     │ read
     ▼
┌────────────────┐      ┌───────┐
│timeline-service│─────▶│ MySQL │ (hydrate wispers)
│    :8082       │      └───────┘
└────┬───────────┘
     │ JSON response
     ▼
┌─────────┐
│ Client  │
└─────────┘
```

## 🌐 Environment Variables

Create a `.env` file in each service directory with the following variables:

```env
# Database Configuration
DB_USER=root
DB_PASS=yourpassword
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=wisper

# Redis Configuration
REDIS_ADDR=localhost:6379

# JWT Secret (must be identical across all services)
JWT_SECRET=your-secret-key-change-in-production

# Service Port (specific to each service)
PORT=8083  # varies by service
```

### Port Assignments
- `auth-service`: `PORT=8083`
- `post-service`: `PORT=8081`
- `timeline-service`: `PORT=8082`
- `follow-service`: `PORT=8085`
- `fanout-worker`: No PORT (background consumer)

## 🚀 Local Development

### Prerequisites
- **Go** 1.22 or higher
- **MySQL** 8.0+
- **Redis** 7.0+

### Setup Instructions

#### 1. Start Infrastructure

**MySQL:**
```bash
# macOS
brew services start mysql@8.0
mysql -u root -h 127.0.0.1 -P 3306

# Linux
sudo systemctl start mysql

# Create database
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS wisper;"
```

**Redis:**
```bash
# macOS
brew services start redis

# Linux
sudo systemctl start redis

# Verify
redis-cli ping  # Should return PONG
```

#### 2. Configure Services

Create `.env` files in each service directory:
```bash
cd auth-service && cp .env.example .env
cd ../post-service && cp .env.example .env
cd ../follow-service && cp .env.example .env
cd ../timeline-service && cp .env.example .env
cd ../fanout-worker && cp .env.example .env
```

Edit each `.env` file with your database credentials and JWT secret.

#### 3. Run Services

**Recommended startup order** (run each in a separate terminal):

```bash
# Terminal 1: Auth Service
cd Backend/auth-service
go run cmd/main.go

# Terminal 2: Follow Service
cd Backend/follow-service
go run cmd/main.go

# Terminal 3: Post Service
cd Backend/post-service
go run cmd/main.go

# Terminal 4: Timeline Service
cd Backend/timeline-service
go run cmd/main.go

# Terminal 5: Fanout Worker
cd Backend/fanout-worker
go run cmd/main.go
```

GORM will automatically create tables on first startup.

## 📡 HTTP APIs (Quick Reference)

### Auth Service (`:8083`)

**Register a new user:**
```http
POST /register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

**Login:**
```http
POST /login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user_id": 1
}
```

### Post Service (`:8081`)

**Create a wisper:**
```http
POST /posts
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json

{
  "content": "Hello Wisper world!"
}

Response:
{
  "id": 123,
  "author_id": 1,
  "content": "Hello Wisper world!",
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Follow Service (`:8085`)

**Follow a user:**
```http
POST /follow
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json

{
  "user_id": 42
}
```

**Unfollow a user:**
```http
POST /unfollow
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json

{
  "user_id": 42
}
```

**Get followers:**
```http
GET /followers/:userId
Authorization: Bearer <JWT_TOKEN>

Response:
{
  "followers": [1, 2, 3, 5, 8]
}
```

**Get following:**
```http
GET /following/:userId
Authorization: Bearer <JWT_TOKEN>

Response:
{
  "following": [42, 43, 44]
}
```

**Check if following:**
```http
GET /is-following/:userId
Authorization: Bearer <JWT_TOKEN>

Response:
{
  "is_following": true
}
```

**Get follow statistics:**
```http
GET /stats/:userId
Authorization: Bearer <JWT_TOKEN>

Response:
{
  "followers_count": 150,
  "following_count": 89
}
```

### Timeline Service (`:8082`)

**Get authenticated user's timeline:**
```http
GET /timeline?cursor=0&limit=20
Authorization: Bearer <JWT_TOKEN>

Response:
{
  "posts": [
    {
      "id": 456,
      "author_id": 42,
      "content": "Great day for coding!",
      "created_at": "2024-01-15T11:00:00Z"
    },
    ...
  ],
  "next_cursor": 1642246800
}
```

**Query Parameters:**
- `cursor` (optional): Unix timestamp for pagination (0 for first page)
- `limit` (optional): Number of wispers to return (default: 20)

## 🗄️ Redis Keys and Streams

### Redis Streams
- **Stream Name**: `post_created_stream`
- **Publisher**: `post-service`
- **Consumer Group**: `fanout_group`
- **Consumer**: `fanout-worker`

**Stream Entry Format:**
```json
{
  "post_id": "123",
  "author_id": "1",
  "created_at": "1642246800"
}
```

### Redis Keys

**Timeline ZSETs:**
- **Key Pattern**: `timeline:{userID}`
- **Members**: Post IDs
- **Scores**: Unix timestamp (for chronological ordering)

Example:
```bash
redis-cli ZREVRANGE timeline:42 0 9 WITHSCORES
```

### Follow Relationships
Stored in MySQL `followers` table:
```sql
CREATE TABLE followers (
  follower_id INT NOT NULL,
  followee_id INT NOT NULL,
  created_at TIMESTAMP,
  PRIMARY KEY (follower_id, followee_id)
);
```

## 🧰 Go Workspace

This repository uses a **Go workspace** (`go.work`) to manage local module dependencies:

```
Backend/
├── go.work          # Workspace definition
├── auth-service/
├── post-service/
├── follow-service/
├── timeline-service/
├── fanout-worker/
└── shared/          # Shared utilities and models
```

All services can import from `shared` without publishing to a remote module registry.

## 🧪 Testing the Complete Flow

### End-to-End Test Scenario

**1. Create two users:**
```bash
# User A
curl -X POST http://127.0.0.1:8083/register \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@wisper.com","password":"alice123"}'

# User B
curl -X POST http://127.0.0.1:8083/register \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@wisper.com","password":"bob123"}'
```

**2. Login and get tokens:**
```bash
# Alice's token
TOKEN_A=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@wisper.com","password":"alice123"}' \
  | jq -r .token)

# Bob's token
TOKEN_B=$(curl -s -X POST http://127.0.0.1:8083/login \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@wisper.com","password":"bob123"}' \
  | jq -r .token)
```

**3. Bob follows Alice:**
```bash
curl -X POST http://127.0.0.1:8085/follow \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_B" \
  -d '{"user_id": 1}'  # Alice's user_id
```

**4. Alice creates a wisper:**
```bash
curl -X POST http://127.0.0.1:8081/posts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_A" \
  -d '{"content":"Hello from Alice! This is my first wisper."}'
```

**5. Bob checks his timeline (should see Alice's wisper):**
```bash
curl -H "Authorization: Bearer $TOKEN_B" \
  "http://127.0.0.1:8082/timeline?limit=20&cursor=0"
```

**6. Verify Redis:**
```bash
# Check stream
redis-cli XINFO STREAM post_created_stream

# Check Bob's timeline
redis-cli ZREVRANGE timeline:2 0 -1 WITHSCORES
```

## ✨ Key Features

### Microservices Architecture
- **Single Responsibility**: Each service handles one domain
- **Independent Deployment**: Services can be deployed separately
- **Language Agnostic**: Easy to add services in other languages
- **Fault Isolation**: One service failure doesn't crash the system

### Event-Driven Design
- **Async Processing**: Post creation doesn't block on fanout
- **Scalability**: Multiple workers can consume the same stream
- **Reliability**: Redis Streams provide durability and replay
- **Decoupling**: Services communicate via events, not direct calls

### Real-time Fanout
- **Instant Distribution**: Wispers appear immediately on follower timelines
- **Push Model**: Pre-computed timelines (write fanout, not read fanout)
- **Scalable**: Works efficiently even with thousands of followers
- **Sorted by Time**: Redis ZSETs maintain chronological order

### Distributed Caching
- **High Performance**: Timeline reads are pure Redis (microsecond latency)
- **Reduced DB Load**: MySQL only hit for wisper hydration
- **Horizontal Scaling**: Redis cluster for massive scale
- **Cache Invalidation**: Simple append-only model

### JWT Authentication
- **Stateless**: No session storage needed
- **Distributed**: Token valid across all services
- **Secure**: HMAC-SHA256 signing
- **Expirable**: Tokens have configurable TTL

### Social Graph Management
- **Bidirectional Queries**: Get followers AND following
- **Fast Lookups**: Indexed queries on MySQL
- **Follow Statistics**: Real-time counts
- **Relationship Checks**: Quick "is following" validation

### Clean Architecture
- **Hexagonal Pattern**: Ports and adapters design
- **Testable**: Business logic separated from infrastructure
- **Maintainable**: Clear separation of concerns
- **Extensible**: Easy to swap implementations

### Independent Scaling
- **Service-Level Scaling**: Scale bottleneck services independently
- **Horizontal Scaling**: Add more instances of any service
- **Worker Scaling**: Add more fanout workers as load increases
- **Database Scaling**: MySQL read replicas, Redis cluster

## 📚 Additional Documentation

- **[RUNBOOK.md](RUNBOOK.md)** - Operations, troubleshooting, and day-2 tasks
- **[SCALABILITY_PLAN.md](SCALABILITY_PLAN.md)** - How to scale to millions of users
- **[frontend-api.md](docs/frontend-api.md)** - Complete API reference for frontend
- **Service READMEs**: Each service has detailed documentation in its directory

## 🐛 Common Issues

See [RUNBOOK.md](RUNBOOK.md) for detailed troubleshooting, but here are quick fixes:

**Services won't start:**
- Check MySQL and Redis are running
- Verify `.env` files exist and are correct
- Ensure ports are not already in use

**Timeline not updating:**
- Verify fanout-worker is running and consuming stream
- Check Redis stream: `redis-cli XINFO STREAM post_created_stream`
- Verify consumer group exists: `redis-cli XINFO GROUPS post_created_stream`

**Authentication fails:**
- Ensure `JWT_SECRET` is identical across all services
- Check token expiration
- Verify `Authorization: Bearer <token>` header format

## 🚀 Production Deployment

For production deployment:
1. Use environment variables for all configuration
2. Enable TLS for all HTTP endpoints
3. Set up proper logging and monitoring
4. Configure Redis persistence (AOF + RDB)
5. Set up MySQL replication for high availability
6. Use a proper secret management system for JWT_SECRET
7. Add rate limiting to prevent abuse
8. Configure proper CORS headers
9. Set up health checks for load balancers
10. Monitor Redis Stream lag and worker performance

See `SCALABILITY_PLAN.md` for architecture at scale.

---

**Built with ❤️ for Wisper** 🌤️