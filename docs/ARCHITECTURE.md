# Architecture

This service follows a **feature-oriented Clean Architecture** with DDD-inspired
domain modeling. It is **gRPC-first**: the proto contract is the source of truth
— it lives in the centralized [`probopass`](https://github.com/kurnhyalcantara/probopass)
contract repo and this service consumes the generated Go stubs as a module
dependency — and REST is derived from it via grpc-gateway. This document is the
blueprint for every service built from this template.

## Layers

```
            ┌────────────────────────────────────────────┐
 inbound →  │ delivery (grpc / rest)  ← transport types  │
            ├────────────────────────────────────────────┤
            │ usecase  ← application logic, ports        │
            ├────────────────────────────────────────────┤
            │ domain   ← entities, invariants (pure)     │
            ├────────────────────────────────────────────┤
 outbound → │ repository ← adapters: pg, redis, APIs ... │
            └────────────────────────────────────────────┘
              platform = infra init   quiver = wiring
```

| Layer | Location | Responsibility |
|---|---|---|
| Domain | `internal/domain/{name}` | Entities, value objects, invariants, domain errors |
| Feature | `internal/features/{name}` | One vertical slice: delivery, usecase, repository, dto, mapper, validator |
| Platform | `platform/*` | Infrastructure **initialization only** (clients, servers, token manager) |
| Quiver | `quiver/*` | Composition root: providers, registries, the container |
| Shared | `pkg/*`, `internal/constants`, `internal/middleware` | Cross-cutting concerns (`pkg/*` is publicly importable) |

## Dependency rules

Enforced by `depguard` in `.golangci.yml` — violations fail `make lint` and CI.

1. **Domain is pure.** `internal/domain/**` imports stdlib and other domain
   packages only. Never transport (the probopass proto stubs, grpc),
   infrastructure (`platform/`, drivers), or features.
2. **Usecases see ports, not adapters.** A usecase imports its domain, its
   feature's `dto` and `repository` *interface*, and `pkg/*`. It must not import
   the probopass proto stubs, `platform/`, drivers, or `delivery`. When a usecase
   needs a platform capability (e.g. token signing), it defines a small interface
   where it is consumed (see `usecase.TokenIssuer`) and quiver injects the
   platform implementation.
3. **Transport types stop at the mapper.** Only `delivery` and `mapper` may
   import the probopass proto stubs. Proto messages never reach usecases or the
   domain.
4. **Platform contains no business logic.** `platform/**` may import `config`
   and third-party libraries, never `internal/` or `quiver/`.
5. **Quiver imports everything; nothing imports quiver** (except `cmd/`).

## Repository definition

A repository is an **outbound adapter abstraction** — the port through which a
usecase reaches anything outside the process:

- PostgreSQL, Redis
- other gRPC/HTTP services
- third-party APIs
- message brokers

It is *not* limited to database access. The interface lives in
`internal/features/{feature}/repository`, named for the capability it provides,
and is consumed by the usecase. Implementations live next to it
(`postgres.go`, `redis_cache.go`, `userservice_grpc.go`, …) and can be composed
— see `NewRedisCache`, a read-through cache decorator that wraps the Postgres
repository while the usecase still sees a single `Repository`.

## Feature structure

```
internal/features/{feature}/
├── delivery/
│   ├── grpc/        # implements the generated *ServiceServer; thin: validate → map → usecase → map
│   └── rest/        # registers the grpc-gateway translation onto the shared mux
├── usecase/         # interface + implementation; application logic and authorization
├── repository/      # outbound port (interface) + adapters
├── dto/             # internal input/output structs (with `validate` tags)
├── mapper/          # pure functions: proto ⇄ dto ⇄ domain
└── validator/       # converts validation failures into apperror.CodeInvalidArgument
```

The matching domain package lives in `internal/domain/{feature}`.

## Dependency injection strategy

Manual constructor wiring — no DI framework, no codegen, no reflection.

- `quiver/provider`: one constructor per dependency. Providers only *construct*;
  composition decisions (e.g. wrapping the Postgres repository in the Redis
  cache) are made here and documented with a comment.
- `quiver/registry`: attaches every feature's handlers to the gRPC server and
  gateway mux, and merges per-feature `ProtectedMethods` for the auth
  interceptor.
- `quiver/container`: `Build(ctx, cfg)` wires config → platform → repositories
  → usecases → handlers → middleware → servers, in that order, and `Close(ctx)`
  releases resources in reverse. `cmd/server` is a cobra CLI; its `serve`
  command loads config, calls `container.Build`, and runs the servers.

## Error handling

- Usecases and repositories return `*apperror.Error`
  (`pkg/apperror`) with a transport-agnostic code; wrapped causes
  are logged but never sent to clients.
- The `middleware.AppError` interceptor maps codes to gRPC statuses; the
  gateway error handler (`middleware.GatewayOptions`) maps those to HTTP with a
  `{"code","message"}` JSON body.
- Repositories translate driver errors to domain errors (`pgx.ErrNoRows` →
  `domain.ErrNotFound`); usecases translate domain errors to apperrors.

## Observability

- **Tracing**: OTel via the otelgrpc stats handler on server and loopback
  client; OTLP export is config-gated (`telemetry.enabled`).
- **Metrics**: OTel Prometheus exporter + Go runtime collectors, scraped from
  `/metrics` on the ops port.
- **Logs**: `slog`, JSON in production; every record is enriched with
  `trace_id`/`span_id` from the context, and every RPC gets one log line with
  method, code, duration, and request id.
- **Health**: gRPC health service; `/healthz` (liveness) and `/readyz`
  (pings Postgres/Redis) on the ops port.

## Naming conventions

- Packages: short, lowercase, singular (`session`, not `sessions`).
- Feature files: `handler.go`, `usecase.go`, `repository.go` (interface),
  `postgres.go`, `redis_cache.go`, `dto.go`, `mapper.go`, `validator.go`.
- Interfaces are named for the role (`Repository`, `Usecase`, `TokenIssuer`);
  implementations are unexported (`postgresRepository`) and returned from
  constructors (`NewPostgres`).
- Protos (in the probopass repo): `proto/probopass/{service}/v1/{service}.proto`,
  package `probopass.{service}.v1`, verb-first RPCs (`CreateSession`), one
  dedicated response message per RPC.
- Migrations: `NNNNNN_description.{up,down}.sql`.
- Env vars: `TEMTEM_` prefix, `__` separates nesting (`TEMTEM_POSTGRES__HOST`).

## Adding a feature (recipe)

1. **Contract**: in the [`probopass`](https://github.com/kurnhyalcantara/probopass)
   repo, add `proto/probopass/{feature}/v1/{feature}.proto` with HTTP
   annotations, run `buf generate`, and release it; then `make proto-update`
   here to pull the generated stubs.
2. **Domain**: create `internal/domain/{feature}` — entity, invariants, errors.
3. **Migration**: `make migrate-create NAME=create_{feature}s`.
4. **Feature slice**: create `internal/features/{feature}/` following the
   session feature layout: dto → repository (interface + adapters) → usecase
   (+ tests against a fake repository) → mapper (+ tests) → validator →
   delivery/grpc (+ `ProtectedMethods`) → delivery/rest.
5. **Wire it**: add providers in `quiver/provider/{feature}.go`, register in
   `quiver/registry` (gRPC, gateway, protected methods), construct it in
   `quiver/container.Build`.
6. **Verify**: `make lint test build` — depguard will flag any layering
   mistake.
