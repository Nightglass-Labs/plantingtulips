# Architecture

## Product shape

PlantingTulips is a focused discovery layer over provider-approved adult-video catalogs. It indexes metadata, builds its own taxonomy/ranking layer, and renders provider embeds. It does not download, proxy, or re-host source video files.

## Production topology

PlantingTulips follows the same application topology as the Nightglass application stack:

```text
SolidJS 2 prerelease + Solid Router 2 + StyleX
                    ↓
                 Vite 8
                    ↓
                   dist
                    ↓ go:embed
            Go 1.25 net/http
        SPA + SEO + /api/v1/*
                    ↓
             Turso / libSQL
                    ↓
          stateless Fly.io
                    ↓
        Cloudflare DNS / proxy
```

There is no Cloudflare Pages Functions runtime and no D1 binding. Cloudflare is an optional edge/DNS layer in front of the Fly origin.

## Frontend

- SolidJS 2 prerelease channel
- `@solidjs/web` DOM renderer
- Solid Router 2 prerelease channel
- Vite 8
- StyleX 0.19

Vite writes `dist/`. The Go binary embeds that directory at build time and serves static assets plus SPA deep-link fallback from the same process.

During development, Vite proxies `/api`, `/robots.txt`, and `/sitemap.xml` to the local Go server.

## Go origin

Go 1.25 uses the standard `net/http` router and exposes a versioned API:

- `GET /api/v1/healthz`
- `GET /api/v1/videos`
- `GET /api/v1/video/{provider}/{id}`
- `GET /api/v1/related/{provider}/{id}`
- `GET /api/v1/categories`
- `POST /api/v1/events`
- `POST /api/v1/report`
- `/api/v1/admin/*` for authenticated operator actions

The Go origin also owns CSP/security headers, initial-HTML SEO metadata, `robots.txt`, `sitemap.xml`, moderation, provider health, catalog ingestion, analytics retention, and SPA fallback.

## Persistence

Turso is the only persistent datastore. The Go app connects through the libSQL database/sql driver using `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`.

SQLite migrations are embedded in the Go binary and applied once using `schema_migrations`. The current catalog/growth schema remains SQLite-compatible, so the D1-era SQL could be retained while the runtime moved to Turso.

Fly Machines are stateless and require no application volume.

## Provider boundary

EPorner uses its official API v2, including search, detail, and removed-ID synchronization. XVideos remains disabled until an approved partner/feed normalization endpoint is configured. Production code does not scrape provider HTML or extract media CDN URLs.

Provider APIs are inputs. Turso is the public catalog system of record, with approved-provider live fallback available when a catalog query is cold.

## Moderation boundary

Local blocks are checked by search, detail, category counts, and recommendations. Reports for suspected minors or NCII can immediately place a local safety hold on the referenced provider/video ID pending operator review.

## Deployment boundary

The production image is multi-stage:

1. Node 24 + pnpm builds the Solid/Vite app.
2. Go 1.25 tests and compiles the server with the generated `dist/` embedded.
3. A distroless non-root image contains only the final binary.

Fly health checks call `/api/v1/healthz`, which also verifies Turso connectivity.
