# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A production-grade Go microservice template — the blueprint for all future services. gRPC-first (protos live in the centralized contract repo [`github.com/kurnhyalcantara/probopass`](https://github.com/kurnhyalcantara/probopass) and are the source of truth; this service imports the generated stubs from there; REST comes from grpc-gateway annotations), feature-oriented Clean Architecture. The `session` feature is the reference implementation every new feature must mirror.

## Commands

- `make build` / `make run` — build / run `cmd/server` (a cobra CLI; `make run` invokes the `serve` subcommand). The binary also exposes `temtem version`; `--config` is a persistent flag.
- `make test` — unit tests; single test: `go test -run TestRefreshRotatesSession ./internal/features/session/usecase/`
- `make test-integration` — requires Postgres/Redis reachable (see below) + migrations first
- `make lint` — golangci-lint; **depguard enforces the architecture rules below, so lint failures may be layering violations, not style**
- `make proto-update` — pull the latest generated stubs from the probopass contract repo (`go get …/probopass@latest && go mod tidy`); edit the `.proto` contracts in that repo, not here
- `make migrate-up` / `make migrate-create NAME=create_foos` — golang-migrate against local Postgres
- `scripts/migrate.sh up` — migrations via Docker when the migrate CLI isn't installed
- Local stack: `deployments/docker-compose.yml` joins the shared external `koer-network` (infernape platform) and talks to its `koer-postgres`/`koer-redis` — it does not start Postgres/Redis itself. `make compose-up` runs temtem against that network; `make compose-migrate` runs migrations the same way.

## Architecture (full rules: docs/ARCHITECTURE.md)

Request flow: `delivery/grpc` (or REST → gateway → loopback gRPC) → `validator` → `mapper` → `usecase` → `repository` (port) → adapter (Postgres/Redis/external service). `container.Build` (root-level `container` package) wires everything by hand (manual DI, no framework) — calling platform and feature constructors directly, no separate provider/registry layer; `cmd/server` is a cobra CLI whose `serve` command loads config, builds the container, and runs the servers (`root.go`/`serve.go`/`version.go`, all `package main`).

Layering rules (depguard-enforced):
- `internal/domain/**` is pure: stdlib + domain packages only.
- `usecase` never imports the probopass proto stubs, `platform/`, drivers, or `delivery`. If a usecase needs a platform capability, define a small interface in the usecase package (see `usecase.TokenIssuer`) and inject it from the container.
- Proto types stop at `mapper`; they never reach usecases or the domain.
- `platform/**` is infrastructure initialization only — must not import `internal/` or `container/`.
- A `repository` is any outbound adapter (DB, cache, other services, brokers), not just database access. Interface in `features/{f}/repository`, implementations beside it, composable (see the Redis read-through decorator `NewRedisCache`).

Cross-cutting:
- Errors: return `*apperror.Error` from usecases/repositories; `middleware.AppError` maps to gRPC codes and the gateway error handler maps those to HTTP JSON. Repositories convert driver errors to domain errors (`pgx.ErrNoRows` → `domain.ErrNotFound`).
- Auth: `middleware.Auth` checks bearer JWTs only for methods listed in each feature's `ProtectedMethods` map (merged in `container.protectedMethods`). Identity travels via `ctxutil.Identity`, not raw claims.
- Config: koanf, precedence defaults < env (`TEMTEM_` prefix, `__` = nesting: `TEMTEM_POSTGRES__HOST`). Environment variables are the single source of truth; copy `.env.example` to `.env` (gitignored, auto-loaded by `make`) for local dev. `--config path.yaml` can still layer in a yaml file for local stacking, but it's optional and loads before env, so env always wins.

## Adding a feature

Follow docs/ARCHITECTURE.md "Adding a feature": add/extend the proto in the probopass repo and release it → `make proto-update` → domain → migration → feature slice copied from `internal/features/session/` → wire it directly in `container.Build` (construct repository/usecase/handler, register on the gRPC server and gateway, add to `protectedMethods`). Usecase tests use a hand-written fake repository (see `usecase_test.go`), not a mocking library.

## Conventions

- Packages singular and lowercase; interfaces named for roles (`Repository`, `Usecase`), implementations unexported with `New*` constructors.
- Tooling notes: this machine has `GOPROXY=direct`; if `go install` of a tool fails on an unreachable vanity import, prefix with `GOPROXY="https://proxy.golang.org,direct"`. The migrate CLI needs `-tags 'postgres'` when installed via `go install`.
