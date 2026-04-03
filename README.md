# VibeChat

A real-time chat backend written in Go with clean architecture, WebSocket messaging, and PostgreSQL.

## Features

- **Real-time messaging** via WebSocket with per-chat rooms and echo prevention
- **REST API** — users, group/direct chats, messages, reactions, read receipts
- **Role-based access control** — owner / admin / member per chat
- **JWT authentication** — access + refresh token pair (HS256)
- **Redis caching** — read-through / write-invalidate cache with configurable TTLs
- **TLS** — native HTTPS/WSS with configurable cert paths
- **pprof profiling** — optional loopback profiling server

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                   Delivery Layer                      │
│         HTTP REST handlers · WebSocket hub            │
├──────────────────────────────────────────────────────┤
│                   Use-Case Layer                      │
│  user · chat · message (port interfaces only)         │
├──────────────────────────────────────────────────────┤
│                   Domain Layer                        │
│  entities · repository interfaces · errors            │
├──────────────────────────────────────────────────────┤
│               Infrastructure Layer                    │
│         postgres · redis cache                         │
└──────────────────────────────────────────────────────┘
```

Dependencies flow inward: infrastructure → domain ← use-case ← delivery.

## Quick Start

```bash
# Start everything (postgres + migrations + app + swagger ui)
docker compose up --build

# Run in background
docker compose up --build -d

# Follow app logs
docker compose logs -f app

# Stop
docker compose down

# Stop and wipe postgres data
docker compose down -v
```

| Service | URL |
|---------|-----|
| API | http://localhost:8080 |
| Swagger UI | http://localhost:8081 |
| Health check | http://localhost:8080/health |

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/register` | Register |
| POST | `/api/v1/auth/login` | Login → token pair |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| GET | `/api/v1/users/me` | Own profile |
| PUT | `/api/v1/users/me` | Update profile |
| GET | `/api/v1/users/search?q=` | Search users |
| GET | `/api/v1/users/{user_id}` | User profile by ID |
| GET | `/api/v1/chats` | Chat list with previews |
| POST | `/api/v1/chats/direct` | Open direct chat |
| POST | `/api/v1/chats/group` | Create group chat |
| GET/PUT | `/api/v1/chats/{id}` | Chat details / update |
| GET/POST | `/api/v1/chats/{id}/members` | Members list / add |
| DELETE | `/api/v1/chats/{id}/members/me` | Leave chat |
| DELETE | `/api/v1/chats/{id}/members/{uid}` | Remove member |
| PATCH | `/api/v1/chats/{id}/members/{uid}/role` | Change member role |
| GET/POST | `/api/v1/chats/{id}/messages` | History / send |
| PUT/DELETE | `/api/v1/chats/{id}/messages/{mid}` | Edit / soft-delete |
| POST/DELETE | `/api/v1/chats/{id}/messages/{mid}/reactions` | React |
| POST | `/api/v1/chats/{id}/read` | Mark read |
| GET | `/health` | Health check |
| GET | `/ws` | WebSocket (requires `Authorization: Bearer` header) |

WebSocket connection is **bidirectional**: the server pushes events to all chat rooms the user belongs to, and the client can send events to any of those rooms. Each client-sent frame must include a `chat_id` field; the server validates membership before broadcasting.

## Configuration

Config files live in `configs/`:

| File | Purpose |
|------|---------|
| `configs/app.yaml` | Runtime config for the API server (Docker) |
| `configs/migrate.yaml` | Runtime config for the migrate tool (Docker) |
| `configs/app.example.yaml` | Annotated template — copy and edit for local dev |
| `configs/migrate.example.yaml` | Annotated template for the migrate tool |

Key fields in `configs/app.example.yaml`:

```yaml
storage:
  type: postgres
  postgres:
    host: localhost
    user: vibechat
    password: <your-db-password>
    dbname: vibechat

jwt:
  access_secret:  "<32+ random chars>"
  refresh_secret: "<32+ different random chars>"

delivery:
  enabled:
    - http
    - ws
  http:
    host: "0.0.0.0"
    port: 8080
    tls:                         # omit section for plain HTTP
      cert_file: certs/cert.pem
      key_file:  certs/key.pem

hasher:
  cost: 12

cache:
  type: mock        # mock (in-process) or redis
  redis:            # required when type is "redis"
    addr: localhost:6379

logger:
  type: stdout      # stdout | console | file
  level: info
```

## Stack

| Concern | Library |
|---------|---------|
| HTTP router | `net/http` (Go 1.22 patterns) |
| WebSocket | `gorilla/websocket` |
| PostgreSQL | `jackc/pgx/v5` |
| Migrations | `golang-migrate/migrate` |
| JWT | `golang-jwt/jwt/v5` |
| Logging | `rs/zerolog` |
| UUID | `google/uuid` |
| Password | `golang.org/x/crypto/bcrypt` |
