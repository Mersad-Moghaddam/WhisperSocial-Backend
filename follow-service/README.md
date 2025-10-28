### follow-service

Manages user follow/unfollow relationships and provides follow-related APIs.

#### Tech
- Go, Fiber
- MySQL via GORM (model: `ports.Follower`)
- JWT authentication via shared middleware

#### Environment
- `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_NAME`
- `JWT_SECRET`

The service loads `../.env` from this folder on startup.

#### Run
```bash
go run cmd/main.go
```
Listens on `:8085`.

#### HTTP API
- POST `/follow`
  - Body: `{ "user_id": number }`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "message": "followed successfully" }`
  
- POST `/unfollow`
  - Body: `{ "user_id": number }`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "message": "unfollowed successfully" }`

- GET `/followers/:userId`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "followers": [1, 2, 3] }`

- GET `/following/:userId`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "following": [1, 2, 3] }`

- GET `/is-following/:userId`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "is_following": true/false }`

- GET `/stats/:userId`
  - Headers: `Authorization: Bearer <token>`
  - 200: `{ "followers_count": 10, "following_count": 5 }`

#### Features
- Prevents self-following
- Checks for duplicate follows/unfollows
- Returns follower/following lists
- Provides follow statistics
- JWT-based authentication

#### Database Model
```sql
CREATE TABLE followers (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    follower_id INT NOT NULL,
    UNIQUE KEY idx_user_follower (user_id, follower_id),
    INDEX idx_user_id (user_id),
    INDEX idx_follower_id (follower_id)
);
```

#### Notes
- All endpoints require JWT authentication
- The authenticated user becomes the follower in follow/unfollow operations
- Follower relationships are used by fanout-worker for timeline distribution