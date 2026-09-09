# PlantingTulips

PlantingTulips is a focused adult-video discovery catalog built from provider-approved metadata feeds and embeds. It does not download, proxy, or re-host source video files.

## Architecture

The production application follows the same architecture as the Nightglass application stack:

```text
SolidJS 2 RC + Solid Router 2 + StyleX
              ↓ Vite 8
             dist/
              ↓ go:embed
        Go 1.25 net/http
         /api/v1/* + SPA
              ↓
         Turso / libSQL
              ↓
       stateless Fly.io
              ↓
   Cloudflare DNS / proxy
```

The Go process is the only application origin. It owns API routing, provider access, catalog ingestion, analytics, moderation, SEO rendering, `robots.txt`, `sitemap.xml`, security headers, and the embedded SPA fallback.

Turso is the only persistent datastore. Fly has no application volume.

## Development

Build the Solid application before starting the embedded Go binary:

```bash
pnpm install --frozen-lockfile
pnpm build:web
TURSO_DATABASE_URL=... TURSO_AUTH_TOKEN=... go run .
```

For frontend iteration, run the Go origin on port 8080 and Vite separately:

```bash
pnpm dev:server
pnpm dev:web
```

Vite proxies `/api`, `/robots.txt`, and `/sitemap.xml` to the Go origin.
