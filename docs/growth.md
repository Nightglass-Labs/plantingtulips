# Growth and revenue foundation

PlantingTulips v0.3 adds first-party measurement, anonymous personalization, SEO landing pages, provider controls, recommendation feedback, and a configurable monetization adapter without changing the provider policy: production media discovery uses only provider-approved APIs, feeds, and embeds.

## First-party analytics

`POST /api/events` stores a random browser-session identifier plus an allow-listed event name and small contextual fields. The application does not intentionally submit an IP address, user-agent string, account identity, or email through this endpoint.

Tracked events cover page/search/category activity, rendered catalog-card impressions, video opens, related impressions/clicks, source clicks, favorites/hides, and ad-slot impressions/clicks. Video engagement rolls into `video_metrics` and feeds catalog/recommendation ranking.

The session ID lives in `sessionStorage`, so it is scoped to the browser session rather than being a durable cross-session identifier. The ingestion endpoint caps request bodies at 8 KiB, validates the session-ID shape, and enforces a per-session event ceiling to prevent trivial D1 write amplification.

Raw `analytics_events` have a 90-day operational retention target. `/api/admin/maintenance` prunes events outside that window, the scheduled catalog workflow runs the same prune daily, and `/admin` exposes the retained count plus oldest/newest event timestamps. Aggregated `video_metrics` and monetization totals are not deleted by the raw-event retention job.

The operator dashboard exposes two distinct seven-day click loops rather than blending them: feed/library card impressions → video opens, and related-card impressions → related clicks. Source-click and favorite rates are measured relative to video opens.

## Anonymous personalization

Favorites, the last 50 viewed videos, hidden items, and preferred-tag weights live in browser `localStorage` under `pt-personalization-v1`. No account is required and this data is not synchronized to the server.

`/library` exposes favorites/history. Watch pages provide Favorite and Hide controls. Preferred tags are used only to improve local navigation.

## SEO

Category, tag, and watch routes still update metadata in the browser for client-side navigation, but v0.3 also includes a Pages Functions edge middleware that rewrites the initial HTML response. Crawlers therefore receive route-specific title, description, canonical URL, robots policy, and OpenGraph metadata without waiting for client JavaScript.

Catalog-backed watch routes additionally receive server-rendered `VideoObject` JSON-LD using the provider embed and catalog thumbnail. Operator/private utility routes such as `/admin`, `/library`, and `/report` are emitted as `noindex,nofollow,noarchive` in the initial response.

`/sitemap.xml` is generated from active categories, popular tags, and recent non-blocked catalog records. `/robots.txt` advertises the deployed sitemap and excludes operator/private utility routes.

## Ranking and deduplication

Catalog ranking combines provider popularity/quality with anonymous PlantingTulips engagement. Related-video ranking considers tag overlap, category overlap, duration similarity, catalog score, opens, related clicks, favorites, and hides.

A normalized title plus five-second duration bucket produces `canonical_key`; public search suppresses duplicate canonical records across providers while retaining the underlying provider rows.

## Provider controls

The `/admin` dashboard can enable or disable supported providers. A disabled provider is excluded from scheduled/manual ingestion and live search/detail fallback. Existing cached rows remain available unless they are separately deactivated or blocked.

XVideos remains disabled by default. It is activated only when an approved partner/feed normalization endpoint is configured and the operator enables the provider. `XVIDEOS_PARTNER_AUTHORIZATION` can carry the exact Authorization header required by that approved endpoint. Do not point these settings at a scraper.

## ExoClick integration

The frontend supports `VITE_AD_MODE=exoclick` using the exact asynchronous script URL and numeric zone IDs supplied by the publisher account. The implementation does not hard-code or guess an ExoClick serving hostname.

Required frontend variables:

- `VITE_EXOCLICK_SCRIPT_URL`
- `VITE_EXOCLICK_ZONE_FEED_BANNER`
- `VITE_EXOCLICK_ZONE_SIDEBAR`
- `VITE_EXOCLICK_ZONE_PLAYER_BELOW`
- `VITE_EXOCLICK_ZONE_NATIVE_CARD`

The dynamically loaded async script is marked `data-cfasync="false"` for Cloudflare Rocket Loader compatibility.

Before enabling `exoclick`, update the production Content-Security-Policy to allow only the exact script/frame/connect origins required by the publisher tag generated for this account. Do not broadly allow arbitrary third-party script hosts.

PlantingTulips currently records its own ad-slot impression count. ExoClick remains the source of truth for billable clicks, impressions, and revenue until authenticated publisher-statistics API synchronization is added. The `monetization_metrics.revenue` column exists for that future synchronization; the application does not invent revenue values.

## Operator dashboard

`/admin` requires `ADMIN_TOKEN` (falling back to `INGEST_SECRET`) and shows catalog size, report totals, last-day events, provider status, seven-day discovery/related funnels, analytics retention, top searches/categories/videos, and monetization counters. It can run a one-page catalog sync, prune raw analytics, and globally enable/disable providers. The token is stored only in `sessionStorage` by the browser UI.

## Production gate

After `migrations/0002_growth.sql` is applied, run the production smoke with both strict modes:

```bash
SITE_URL=https://your-domain.example REQUIRE_CATALOG=1 REQUIRE_GROWTH=1 pnpm smoke:production
```

The growth smoke verifies that HTML routes pass through the edge SEO middleware, private routes are `noindex`, a catalog watch route contains server-rendered `VideoObject` metadata, D1-backed analytics accepts an event, and the normal catalog/detail/related checks still pass.
