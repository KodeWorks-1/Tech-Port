# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

TechPort Store — an e-commerce site for tech accessories (Pakistan). Go server rendering `html/template` pages progressively enhanced with HTMX/Alpine, backed by PostgreSQL (pgx). No JS build step, no ORM, no test suite currently. Strategy lives in `techport-store-plan.md`, engineering plan in `build-plan.md`.

## Commands

```sh
docker compose up -d db   # local postgres on localhost:5543 (or: just db-up)
go run ./cmd/server       # migrates + seeds automatically, serves :8080 (or: just dev)
go build ./...            # compile check
go test ./...             # (no tests exist yet; or: just test)
```

Config comes from `.env` (loaded by a tiny loader in `internal/config`; existing env vars win). Defaults are dev-friendly: `ENV=dev`, `PORT=8080`, and a `DATABASE_URL` pointing at the docker-compose postgres — so `go run ./cmd/server` works with zero setup after `db-up`.

Admin UI at `/admin`; first seed creates `admin@techport.pk` / `changeme123`.

## Architecture

**Two entrypoints, one router.** `cmd/server/main.go` is the normal server; `api/index.go` wraps the identical setup as a single Vercel serverless function (`vercel.json` rewrites all paths to it). Both do: config → `db.Connect` → `db.Migrate` → `db.SeedIfEmpty` → construct all services → `handlers.New(...).Router()`. **Any new service must be wired in both files.**

**Layering:** `internal/handlers` (chi routes + HTTP concerns) → `internal/services` (one struct per domain — Catalog, Cart, Orders, Users, Settings, Admin, AdminAuth — each holding the pgx pool and writing SQL directly) → Postgres. Domain structs live in `internal/models`.

**Migrations** are plain SQL files in `migrations/` embedded into the binary and applied at startup, tracked in `schema_migrations` by the integer filename prefix. To change schema, add `00000N_name.up.sql` + `.down.sql` — never edit an applied migration. Seeding (`internal/db/seed.go` + `catalog.json`) only runs when the products table is empty.

**Rendering** (`internal/handlers/render.go`): every page renders inside a layout chosen by page-name prefix — `"admin/x.html"` → `views/admin-layout.html` + `views/admin/x.html`, everything else → `views/layout.html` + `views/pages/x.html`. All of `views/partials/*.html` is parsed into every template set; `RenderPartial` executes one by name without a layout for HTMX fragment swaps (cart box, product cards, etc.). In dev (`ENV=dev`) templates re-parse on every request, so template edits show up on refresh without restarting.

Template funcs to know: `money`/`moneyPtr` (float rupees → "Rs. 3,000" — prices are float64 rupees throughout), `pct` (discount % from compare-at price), `assetVer` (cache-busts `app.css` by modtime). Nav data (categories, settings) is injected as template funcs via `handlers.NavFuncs`.

**Sessions & auth are three separate mechanisms:** a `tp_session` cookie (set by the `session` middleware) keys guest carts; customer accounts (`internal/services/users.go`, `requireUser` middleware); admin sessions stored in the DB (`admin_auth.go`, `requireAdmin` middleware).

**Orders** have a status flow with an `order_events` audit trail; cancelling/returning an order automatically restocks variants (see `internal/services/orders.go` / `admin_store.go`).

## Deploy notes

- Vercel: read-only filesystem, so admin image uploads don't work there; `POSTGRES_URL` (Neon integration) is accepted as an alias for `DATABASE_URL`.
- Render blueprint (`render.yaml`) and Docker (`compose.prod.yml`) also supported; uploads persist only in the Docker `uploads` volume.
- Guest-facing product/category/home pages are safe to full-page cache; purge on admin edits.
