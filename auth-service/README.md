### auth_service

Handles user registration and JWT-based login.

#### Tech
- Go, Fiber
- MySQL via GORM (model: `ports.User`)
- JWT via `shared/config/jwt.go`

#### Environment
- `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_NAME`
- `JWT_SECRET`

The service loads `../.env` from this folder on startup.

#### Run
```bash
go run cmd/main.go
```
Listens on `:8083`.

#### HTTP API
- POST `/register`
  - Body: `{ "email": string, "password": string }`
  - 200: `{ "message": "User registered" }`
- POST `/login`
  - Body: `{ "email": string, "password": string }`
  - 200: `{ "token": string }`

#### Notes
- Passwords are hashed with bcrypt.
- Tokens expire in 72h. Secret comes from `JWT_SECRET`.
