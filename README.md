# temtem

Production-grade Go microservice template — the standard blueprint for new
services. Feature-oriented Clean Architecture, gRPC-first with grpc-gateway
REST, PostgreSQL, Redis, JWT auth, OpenTelemetry + Prometheus, Docker, and
GitHub Actions CI.

The repository ships with one complete example feature — `session`
(authentication sessions: create/get/refresh/revoke with refresh-token
rotation) — exercising every layer of the architecture. Read
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the rules; copy the session
feature's shape for new features.

## Requirements

- Go 1.25+
- Docker (Postgres/Redis via compose)
- Proto contracts live in [`github.com/kurnhyalcantara/probopass`](https://github.com/kurnhyalcantara/probopass);
  this service imports the generated Go stubs as a module dependency. `make
  proto-update` pulls the latest.
- `make tools` installs golangci-lint and migrate

## Quickstart

```sh
docker compose -f deployments/docker-compose.yml up -d postgres redis
scripts/migrate.sh up        # or: make migrate-up (needs the migrate CLI)
make run
```

Smoke test over REST (gateway on :8080):

```sh
# login → returns session + access/refresh tokens
curl -s -X POST localhost:8080/v1/sessions -d '{"user_id":"u-123"}'

# authenticated read
curl -s localhost:8080/v1/sessions/<session_id> -H "Authorization: Bearer <access_token>"

# rotate the refresh token
curl -s -X POST localhost:8080/v1/sessions/refresh -d '{"refresh_token":"<refresh_token>"}'

# logout
curl -s -X DELETE localhost:8080/v1/sessions/<session_id> -H "Authorization: Bearer <access_token>"
```

gRPC is on :9090 (reflection enabled — `grpcurl -plaintext localhost:9090 list`).
Ops endpoints on :9100: `/metrics`, `/healthz`, `/readyz`.

## Commands

| Command | Purpose |
|---|---|
| `make run` / `make build` | run / build `cmd/server` |
| `make test` | unit tests (race + coverage) |
| `make test-integration` | integration tests (needs compose services + migrations) |
| `make lint` | golangci-lint, including depguard architecture rules |
| `make proto-update` | pull the latest generated stubs from the probopass contract repo |
| `make migrate-up` / `make migrate-down` | apply / roll back one migration |
| `make migrate-create NAME=...` | create a new migration pair |
| `make docker-build` | build the production image |
| `make docker-up` / `make docker-down` | start / stop compose services |

## Project structure

```
cmd/server/       cobra CLI: serve (config → container → run) + version
config/           config loader (defaults < yaml < TEMTEM_* env)
internal/
  domain/         pure domain models and invariants
  features/       one vertical slice per feature (delivery/usecase/repository/dto/mapper/validator)
  middleware/     gRPC interceptors + gateway options
  constants/      app-wide constants
pkg/              publicly importable library code (apperror, ctxutil, pagination)
platform/         infrastructure initialization only (pg, redis, grpc, jwt, logger, telemetry, validator)
quiver/           composition root (provider, registry, container)
migrations/       golang-migrate SQL files
deployments/      Dockerfile + docker-compose
test/integration/ integration tests (`-tags=integration`)
docs/             architecture guidelines
```

Proto contracts and their generated Go stubs live in the centralized
[`probopass`](https://github.com/kurnhyalcantara/probopass) repository and are
consumed as a module dependency.

## Configuration

Precedence: defaults < `config/config.yaml` < environment. Env convention:
`TEMTEM_` prefix, `__` for nesting — `TEMTEM_POSTGRES__HOST=db` overrides
`postgres.host`. See `config/config.example.yaml`. The server refuses to start in
production with the default JWT secret.

## Using this template for a new service

1. Copy the repo; replace module path `github.com/kurnhyalcantara/temtem` and
   the `TEMTEM_` env prefix (`config/config.go`).
2. Rename/replace the `session` feature with your first real feature following
   [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#adding-a-feature-recipe).
3. Update `app.name` in config and the compose/CI image names.
