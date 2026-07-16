# TechPort Store — Build Plan

**Date:** 2026-07-16 · Companion to `techport-store-plan.md` (product/strategy doc)

---

## 1. Stack decision

**Go (server-rendered) + HTMX + Alpine.js + Tailwind CSS + PostgreSQL, single binary on our VPS.**

Why this beats the alternatives for "everything on our server + very good UX":

- **UX in e-commerce = speed.** Server-rendered HTML from a Go binary gives sub-100ms page renders; with `hx-boost` navigation the store feels like an SPA (no full page reloads) without shipping a JS bundle. Product pages arrive complete — no spinners, no layout shift. This is the same reason dressify feels fast.
- **Team leverage.** We reuse proven dressify patterns: project layout, migrations, image handling, Docker/CI deploy, Cloudflare setup. Fastest path to shipping, lowest bus factor.
- **Self-host friendly.** One binary + Postgres in Docker compose. No Node runtime, no build-server memory issues on a small VPS, trivial rollbacks.
- **SEO-native.** Real HTML pages with schema.org markup — no SSR framework gymnastics.
- Rejected: **Next.js** (great UX but a second ecosystem to maintain, heavier on the VPS, overkill for tens of SKUs), **Shopify/Woo** (ruled out in strategy doc).

### Components

| Layer | Choice |
| --- | --- |
| HTTP | Go 1.22+, chi router (or stdlib mux), html/template |
| Interactivity | HTMX (cart, variant picker, filters, admin tables) + Alpine.js (gallery, drawers, toasts) |
| Styling | Custom CSS design system (tokens + components in `static/css/app.css`) — zero toolchain; Tailwind optional later if the team prefers |
| DB | PostgreSQL 16, pgx, golang-migrate |
| Images | Upload → resize to WebP variants (thumb/card/zoom) → serve from disk behind Cloudflare cache |
| Sessions/cart | Signed cookie session ID → cart rows in Postgres |
| Auth | Admin: username+password (bcrypt) + session; customers: phone-first, no account required to order |
| Analytics | PostHog EU (same taxonomy approach as dressify) |
| Deploy | Docker compose (app + postgres + caddy) on Contabo Singapore VPS, GitHub Actions CI/CD, Cloudflare in front |

## 2. UX principles (what "very good" means concretely)

1. **Instant feel:** `hx-boost` page transitions, prefetch on hover, optimistic cart badge updates.
2. **Zero jank:** fixed image aspect boxes (no layout shift), LQIP/blur placeholders, lazy loading below the fold.
3. **Mobile-first:** most PK traffic is mobile; sticky add-to-cart bar, thumb-reachable checkout, drawer cart.
4. **Checkout in one page:** name, phone, address, city, payment method — no account creation, no multi-step wizard. Phone number is the identity.
5. **Full-page Cloudflare cache for guests** on home/category/product pages, purged on admin edits → PK users get ~20ms cached loads despite SG origin.
6. **Trust everywhere:** delivery promise, return policy, WhatsApp button, Daraz link on every product page.

## 3. Data model (first cut)

- `categories` (slug, name, parent_id, sort)
- `products` (slug, title, brand, category_id, description, specs JSONB, base_price, compare_at_price, active, seo fields)
- `product_variants` (product_id, options JSONB e.g. {color}, price, stock, sku)
- `product_images` (product_id, variant_id?, path, alt, sort)
- `carts` / `cart_items` (session_id, variant_id, qty, price_at_add)
- `orders` (code e.g. TP-1024, customer name/phone/address/city, payment_method, status: pending→confirmed→shipped→delivered / cancelled / returned, subtotal, shipping_fee, total, notes)
- `order_items` (order_id, variant_id, title snapshot, price snapshot, qty)
- `order_events` (order_id, status, note, actor, at) — audit trail powering the tracking page
- `admin_users` (email, password_hash, role)
- `settings` (key/value: shipping fee, WhatsApp number, payment methods on/off + "coming soon" flags)

## 4. Pages

**Storefront:** home (hero + featured + categories) · category listing (sort, price filter) · product page (gallery, variant picker, specs table, related) · cart · checkout (single page, all payment methods shown, only COD enabled) · order confirmation · order tracking (lookup by order code + phone) · about / warranty & returns / contact.

**Admin (`/admin`):** dashboard (today/week orders, revenue, low-stock) · orders list + detail with status flow buttons · product CRUD with image upload & variant editor · categories · settings.

## 5. Milestones

| # | Days | Deliverable |
| --- | --- | --- |
| M1 | 1–3 | Repo scaffold, Docker compose, migrations, base layout + design system, seed with sample products |
| M2 | 4–7 | Storefront: home, category, product page (variants, gallery, specs), cart |
| M3 | 8–10 | Checkout (all payment methods displayed, COD simulated end-to-end), confirmation + tracking pages |
| M4 | 11–14 | Admin dashboard: orders + status flow, product/category CRUD, image pipeline, settings |
| M5 | 15–17 | Polish: SEO (schema.org, OG, sitemap), performance pass, mobile QA, PostHog, deploy to Contabo SG + Cloudflare cache rules → demo to customer |

Real catalog import (Seller Center export) drops in whenever it arrives — seed data carries us until then.

## 6. Immediate next steps

1. Scaffold Go module + project layout in `tech port` (`cmd/server`, `internal/{handlers,services,models}`, `migrations`, `views`, `static`)
2. Docker compose for local dev (app + Postgres)
3. Migration 000001 (schema above), seed script with ~10 sample tech-accessory products incl. the CoolBell keyboard
4. Base layout: Tailwind setup, header/footer/nav, design tokens (colors, type scale)
