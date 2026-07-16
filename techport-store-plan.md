# TechPort — Online Store: Research & Build Plan

**Date:** 2026-07-16
**Customer:** "TechPort." — Daraz seller (tech accessories)
**Daraz shop:** https://www.daraz.pk/shop/ge49t4yv/ (shopId 1752887, sellerId 6005224288194, ships from Punjab)

---

## 1. What we know about the customer

- New/small Daraz shop selling tech accessories.
- Verified listing: **CoolBell Smart Board BT-KB02** (Bluetooth keyboard + touchpad) at **Rs. 3,000** — "No Brand", no ratings/sales yet; undercuts a competitor ("Ali - Computer") selling the same item at Rs. 3,499.
- Expected catalog: tens of SKUs in the Rs. 1,000–5,000 range (keyboards, mice, chargers, hubs, etc.).
- Daraz shop catalogs sit behind signed APIs — scraping is impractical.
  **→ Get the product list from Daraz Seller Center: Products → Manage Products → Export (Excel).** That file becomes our catalog import source.

## 2. Why a standalone store (Daraz stays as a channel)

- **Margin:** Daraz commission + payment fees eat a real slice of a Rs. 3,000 item. Own store keeps it.
- **Customer ownership:** phone numbers / WhatsApp list for restock & new-arrival broadcasts — Daraz never gives you the customer.
- **Trust & brand:** spec-complete pages, warranty/returns policy, real photos — things Daraz listings do badly.
- Daraz remains for discovery/volume; the store is for margin and repeat buyers.

## 3. Platform decision

**Custom build on our Go stack (dressify patterns)** — recommended.

| Option | Verdict |
| --- | --- |
| Custom Go + Postgres + server-rendered templates | ✅ Reuses proven catalog/variant/deploy patterns; no monthly platform fee; becomes our reusable store platform for future clients |
| Shopify (~$29+/mo + apps) | Fast but recurring USD cost, limited customization, makes us replaceable |
| WooCommerce | Cheap but a maintenance liability; below team skill level |

No search engine needed at this scale (Postgres is plenty). Simple category nav.

## 4. Payments (Pakistan reality)

**MVP approach: show every payment method at checkout, but only COD actually works.**
The checkout UI displays all options — COD, credit/debit card, Easypaisa, JazzCash, bank transfer — so the customer sees the full vision; non-COD methods are visible but marked "coming soon" (or shown in a demo/disabled state). COD runs as a working simulation end-to-end: place order → order confirmation → status flow, without a live courier hookup yet.

Rollout order after feedback:
1. **COD first-class** — majority of PK e-commerce orders. Add a WhatsApp/phone confirmation step to filter fake orders.
2. **Wallets:** Easypaisa / JazzCash (lowest MDR ~1.5–2.5%) — manual or API.
3. **Card gateway:** Safepay (developer-friendly, SBP-regulated) or XPay (embedded checkout, pairs with PostEx).
4. Bank transfer with manual confirmation as an option for higher-ticket items.

## 5. Delivery & COD logistics

- **MVP:** COD is simulated — orders land in the admin dashboard with a manual status flow (pending → confirmed → shipped → delivered / returned); the customer books couriers however they do today.
- **Post-MVP — PostEx** as primary courier: ~70% national COD parcel share, developer-friendly API, COD remittance/reconciliation, reverse logistics, advances COD cash to merchants.
- Target flow: order confirmed → book shipment via API → push tracking to customer (WhatsApp/SMS) → reconcile COD remittance.
- Backup couriers later: Trax, Leopards, TCS.

## 6. Store requirements (tech-products specifics)

- Spec-driven product pages with structured attributes; comparison-friendly.
- Variant support (color/size combos, per-variant price + stock) — reuse dressify options/variants JSONB work.
- Trust signals (critical for a no-ratings seller): real photos, warranty & return policy page, delivery promises, visible WhatsApp number, Daraz shop link as social proof.
- **WhatsApp everything:** order-on-WhatsApp button, order confirmations, broadcast list.
- SEO from day one: schema.org Product markup, clean URLs — accessory queries ("bluetooth keyboard with touchpad price in pakistan") are low-competition.

## 7. MVP scope

**Strategy: build the whole store as an MVP first, ship it, then add features based on customer feedback.**

1. Full storefront: home, category pages, product pages (variants + spec tables), cart, checkout.
2. Checkout shows **all** payment methods (COD, credit/debit card, Easypaisa, JazzCash, bank transfer) — **only COD functional (simulated)**, the rest visible as "coming soon".
3. **Admin dashboard**: manage products / stock / prices / images, view + update orders (status flow: pending → confirmed → shipped → delivered / returned), basic sales overview.
4. Host on Contabo Singapore VPS behind Cloudflare (see §8).

**Post-MVP (driven by feedback):** live payment gateways (Safepay/XPay, wallet APIs), PostEx courier integration, WhatsApp order notifications, BNPL/installments (KalPay/QistPay), Daraz inventory sync, reviews, multi-courier routing.

## 8. Hosting

**Decision (2026-07-16): most probably Contabo Singapore VPS.**

- We already run infra on Contabo (known pricing, reliability, workflow) and have an SG VPS in use — familiar territory, no new provider risk.
- Behind Cloudflare (PK PoPs in Karachi/Lahore/Islamabad), static/cached content stays local; only dynamic requests pay the PK→SG round-trip (~90–120 ms) — acceptable for an MVP at this scale.
- Aggressive Cloudflare caching (full-page cache for product/category pages, purge on admin edits) narrows the gap further.

### Alternative: Pakistan-local VPS (researched 2026-07-16, keep as post-MVP option if latency feedback is bad)

Local PK origin cuts dynamic latency to ~5–20 ms nationwide. PK→India routing is notoriously indirect, so Mumbai is not the shortcut it looks like.

| Provider | Location | Reference plan | Price |
| --- | --- | --- | --- |
| Virtury Cloud (virtury.com) | Islamabad | 4 vCPU / 8 GB / 160 GB NVMe / 6 TB | $40/mo (smallest $10/mo) |
| CloudVPS.pk | ISB / KHI / LHE | 4 core / 8 GB / 160 GB NVMe / 6 TB (ISB) | Rs. 11,000/mo, pays via Easypaisa/JazzCash |
| FussionHost | Karachi | 4 core / 8 GB (only 50 GB disk) | $39.99/mo |
| RapidCompute (Cybernet) | 3× KHI + LHE | Custom, PAYG | Quote — enterprise/reliability pick |
| Nayatel | Islamabad | Custom | Quote |

Local hosts carry reliability risk (power/upstream) — if we ever switch, trial Virtury or CloudVPS.pk Islamabad first.

## 9. Open items

- [ ] Get Daraz Seller Center product export from customer
- [ ] Confirm stack choice with team
- [ ] Confirm domain name + branding assets
- [x] Hosting: Contabo Singapore VPS (behind Cloudflare) — decided 2026-07-16
- [ ] Post-MVP: customer's PostEx merchant account (or sign up)
- [ ] Post-MVP: Easypaisa/JazzCash + card gateway merchant details
