# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A production-grade Go microservice template — the blueprint for all future services. gRPC-first (protos live in the centralized contract repo [`github.com/kurnhyalcantara/probopass`](https://github.com/kurnhyalcantara/probopass) and are the source of truth; this service imports the generated stubs from there; REST comes from grpc-gateway annotations), layered Clean Architecture organized **by responsibility, not by feature**: `internal/{domain,usecase,repository,validator,handler}` (with `handler/dto` and `handler/mapper`). The `example` CRUD slice is the reference implementation every new capability must mirror.

## Commands

- `make build` / `make run` — build / run `cmd/server` (a cobra CLI; `make run` invokes the `serve` subcommand). The binary also exposes `temtem version`; `--config` is a persistent flag.
- `make test` — unit tests; single test: `go test -run TestListPaginates ./internal/usecase/`
- `make test-integration` — requires Postgres/Redis reachable (see below) + migrations first
- `make lint` — golangci-lint; **depguard enforces the architecture rules below, so lint failures may be layering violations, not style**
- `make proto-update` — pull the latest generated stubs from the probopass contract repo (`go get …/probopass@latest && go mod tidy`); edit the `.proto` contracts in that repo, not here
- `make migrate-up` / `make migrate-create NAME=create_foos` — golang-migrate against local Postgres
- `scripts/migrate.sh up` — migrations via Docker when the migrate CLI isn't installed
- Local stack: `deployments/docker-compose.yml` joins the shared external `koer-network` (infernape platform) and talks to its `koer-postgres`/`koer-redis` — it does not start Postgres/Redis itself. `make compose-up` runs temtem against that network; `make compose-migrate` runs migrations the same way.

## Architecture (full rules: docs/ARCHITECTURE.md)

Request flow: `handler` (gRPC, or REST → gateway → loopback gRPC) → `validator` → `mapper` → `usecase` → `repository` (port) → adapter (Postgres/Redis/external service). `container.Build` (root-level `container` package) wires everything by hand (manual DI, no framework) — calling platform and layer constructors directly, no separate provider/registry layer; `cmd/server` is a cobra CLI whose `serve` command loads config, builds the container, and runs the servers (`root.go`/`serve.go`/`version.go`, all `package main`).

Layering rules (depguard-enforced):
- `internal/domain/**` is pure: stdlib + domain packages only.
- `usecase` never imports the probopass proto stubs, `platform/`, drivers, or `internal/handler` (including `handler/dto` and `handler/mapper`). It takes primitive/domain inputs and returns domain or usecase-owned types; the `dto` package is a handler concern. If a usecase needs a platform capability, define a small interface in the usecase package and inject it from the container.
- Proto types stop at `handler`/`handler/mapper`; they never reach usecases or the domain.
- `platform/**` (in `kingler`) is infrastructure initialization only — must not import `internal/` or `container/`. Platform constructors take a single `Config` struct (the source of truth), not functional options — e.g. `postgres.New(ctx, postgres.Config{DSN: …})`.
- A `repository` is any outbound adapter (DB, cache, other services, brokers), not just database access. Interface in `internal/repository`, implementations beside it, composable (see the Redis read-through decorator `NewRedisCache`).

Cross-cutting:
- Errors: return `*apperror.Error` from usecases/repositories; `middleware.AppError` maps to gRPC codes and the gateway error handler maps those to HTTP JSON. Repositories convert driver errors to domain errors (`pgx.ErrNoRows` → `domain.ErrNotFound`).
- Config: koanf, precedence defaults < env (`TEMTEM_` prefix, `__` = nesting: `TEMTEM_POSTGRES__HOST`). Environment variables are the single source of truth; copy `.env.example` to `.env` (gitignored, auto-loaded by `make`) for local dev. `--config path.yaml` can still layer in a yaml file for local stacking, but it's optional and loads before env, so env always wins.

## Adding or changing a capability

Follow docs/ARCHITECTURE.md "Adding or changing a capability": add/extend the proto in the probopass repo and release it → `make proto-update` → domain → migration → extend each layer following the `example` slice (`repository` → `usecase` → `handler/dto` → `handler/mapper` → `validator` → `handler`) → wire it directly in `container.Build` (construct repository/usecase/handler, register on the gRPC server and gateway). Usecase tests use a hand-written fake repository (see `internal/usecase/usecase_test.go`), not a mocking library.

## Conventions

- Packages singular and lowercase; interfaces named for roles (`Repository`, `Usecase`), implementations unexported with `New*` constructors.
- Tooling notes: this machine has `GOPROXY=direct`; if `go install` of a tool fails on an unreachable vanity import, prefix with `GOPROXY="https://proxy.golang.org,direct"`. The migrate CLI needs `-tags 'postgres'` when installed via `go install`.
