# Social Network — Backend API Reference (Frontend)

This document summarizes the backend HTTP APIs available to the Front-end team and includes example curl commands and a Postman collection you can import.

Location: `docs/frontend-api.md`

## Quick service base URLs (local development)

- Auth: `http://localhost:8080`

- Follow: `http://localhost:8082`

- Notification: `http://localhost:8083` (routes are under `/notifications`)

- Post: `http://localhost:8084`

- Timeline: `http://localhost:8085`

Each service exposes a `/health` endpoint.

---

## Common conventions

- Authentication: JWT Bearer tokens in header `Authorization: Bearer <token>`.

- CSRF: if required, header `X-CSRF-Token` and cookie `_csrf_token` (security middleware defaults).

- Rate limiting: responses may include `X-RateLimit-*` and `Retry-After` header for 429 responses.

- Error format: usually JSON with keys such as `error`, `message`, and sometimes `code`.

---

## Endpoints and examples

For all examples replace `<TOKEN>` with a valid JWT access token.

### 1) Auth service

- POST `/register`
  - Body: `{ "email": "you@example.com", "password": "secret" }`
  - Success: `200 { "message": "user registered" }`

Example curl:

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret"}'
```

- POST `/login`
  - Body: `{ "email": "you@example.com", "password": "secret" }`
  - Success: `200 { "token": "<JWT>" }`

Example curl:

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret"}'
```

Save the returned `token` and include it as `Authorization: Bearer <token>` for protected endpoints.

---

### 2) Post service

- POST `/posts`  (create a post)
  - Auth required
  - Body: `{ "content": "Hello world" }`
  - Validation: non-empty, max 2000 chars
  - Success: returns created post object

Example curl:

```bash
curl -X POST http://localhost:8084/posts \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"content":"My first post from UI"}'
```

---

### 3) Follow service

- POST `/follow`
  - Auth required
  - Body: `{ "user_id": 123 }`
  - Success: `{ "message": "followed successfully" }`

Example curl:

```bash
curl -X POST http://localhost:8082/follow \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"user_id":123}'
```

- POST `/unfollow`
  - Same shape as `/follow`, returns `{ "message": "unfollowed successfully" }`

- GET `/followers/:userId`
  - Returns `{ "followers": [...] }`

Example curl (followers):

```bash
curl http://localhost:8082/followers/123
```

- GET `/is-following/:userId`
  - Auth required
  - Returns `{ "is_following": true|false }`

Example curl (is-following):

```bash
curl -H "Authorization: Bearer <TOKEN>" http://localhost:8082/is-following/123
```

- GET `/stats/:userId`
  - Returns follower/following counts: `{ "followers_count": N, "following_count": M }`

---

### 4) Timeline service

- GET `/timeline`
  - Auth required
  - Query params:
    - `cursor` (optional, default 0) — integer cursor
    - `limit` (optional, default 10, max 50)
  - Returns: `{ "posts": [...], "next_cursor": <int64> }`

Example curl:

```bash
curl -H "Authorization: Bearer <TOKEN>" "http://localhost:8085/timeline?limit=10"
```

Notes:

- Cursor-based pagination. Use `next_cursor` to fetch the next page.

- Timeline endpoints are rate-limited — respect `X-RateLimit-*` headers.

---

### 5) Notification service

Base path: `/notifications`

- GET `/notifications` (query params: `limit`, `cursor`, `type`)
  - Auth required
  - Example response: `{ "data": [...], "pagination": { "next_cursor": "..." } }`

Example curl:

```bash
curl -H "Authorization: Bearer <TOKEN>" "http://localhost:8083/notifications?limit=20"
```

- PUT `/notifications/:id/read`
  - Auth required
  - Marks a notification as read. Returns `{ "message": "Notification marked as read" }`

Example curl:

```bash
curl -X PUT http://localhost:8083/notifications/42/read \
  -H "Authorization: Bearer <TOKEN>"
```

- DELETE `/notifications/:id`
  - Auth required
  - Deletes a notification.

Example curl:

```bash
curl -X DELETE http://localhost:8083/notifications/42 \
  -H "Authorization: Bearer <TOKEN>"
```

---

## Error handling guidance for UI

- 400: validation or bad request — show field-level error if possible.

- 401: unauthorized — clear client auth and redirect to login.

- 403: forbidden — show appropriate permission message.

- 404: resource not found.

- 409: conflict (e.g., register existing email).

- 429: rate limited — respect `Retry-After` and `X-RateLimit-*` headers.

## Front-end best practices

- Centralize API calls with a small HTTP client wrapper that:

  - Adds `Authorization` header when token is present.

  - Adds `X-CSRF-Token` header for mutating requests if cookie `_csrf_token` exists.

  - Handles 401 globally (redirects to login or triggers refresh attempt).

  - Retries idempotent GETs on transient errors with backoff.

- For timeline and notifications use cursor-based infinite scroll.

- Use optimistic UI updates for follow/unfollow and post creation, but reconcile from server response.

---

## Postman collection

Import `docs/postman_collection.json` in Postman to get working examples for the endpoints listed above. Replace the `{{baseUrl}}` and `{{token}}` environment variables with your values.
