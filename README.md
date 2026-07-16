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

## Admin

`/admin` — first run seeds `admin@techport.pk` / `changeme123` (**change this
before going live**). Manages orders (status flow with automatic restock on
cancel/return), products, variants, images, categories, and store settings
(shipping fee, WhatsApp number, payment method visibility).

## Free demo hosting (Render)

`render.yaml` is a ready Render blueprint: create a free account at
render.com, **New + → Blueprint**, pick this GitHub repo, Apply. It builds
the Dockerfile, provisions a free Postgres, and the app migrates + seeds
the real catalog on first boot. You get a public
`https://techport-xxxx.onrender.com` URL to share with the client.

Free-tier caveats: the service sleeps after ~15 min idle (first request
takes ~50 s to wake — open it before the demo), uploads don't persist
across restarts, and the free database expires after 30 days.

## Deploy (Contabo SG or any Docker host)

```sh
cp .env.production.example .env.production   # edit DATABASE_URL password
POSTGRES_PASSWORD=... docker compose -f compose.prod.yml up -d --build
```

App listens on 127.0.0.1:8080 — put Cloudflare in front (proxied DNS +
origin rule) or Caddy/nginx for TLS. Product/category/home pages are safe to
full-page cache for guests; purge on admin edits. Migrations and seed run
automatically at startup. Uploaded product images persist in the `uploads`
volume.

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
