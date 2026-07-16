# TechPort Store

Online store for TechPort (tech accessories, Pakistan). Go + server-rendered
templates + HTMX, PostgreSQL. See `techport-store-plan.md` (strategy) and
`build-plan.md` (engineering plan).

## Local development

```sh
docker compose up -d db   # postgres on localhost:5543
go run ./cmd/server       # migrates + seeds automatically, serves :8080
```

Or with [just](https://github.com/casey/just): `just db-up`, `just dev`.

Config via `.env` (copy from `.env.example`). A fresh database is seeded with
demo products so the store is browsable immediately.

## Layout

- `cmd/server` — entrypoint (config, DB connect, migrate, seed, HTTP)
- `internal/config` — env config + tiny .env loader
- `internal/db` — pool, embedded-migration runner, demo seed
- `internal/models` — domain structs
- `internal/services` — DB-backed business logic (catalog, …)
- `internal/handlers` — chi router, page handlers, template renderer
- `migrations` — SQL migrations (embedded into the binary)
- `views` — layout / pages / partials (html/template)
- `static` — CSS design system, htmx/alpine, product images
