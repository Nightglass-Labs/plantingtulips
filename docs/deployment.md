# Fly + Turso deployment

PlantingTulips runs as one stateless Go 1.25 application on Fly.io. The Vite-built SolidJS 2 SPA is compiled into `dist/` and embedded into the Go binary with `go:embed`. Turso/libSQL is the only persistent database.

Cloudflare may proxy the public hostname, but it is not the application runtime and no D1/Pages Functions configuration is required.

## 1. Turso

Create or select a production Turso database and obtain:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

Do not manually apply the application migrations in normal operation. The Go server embeds `migrations/*.sql`, records them in `schema_migrations`, and applies each unapplied migration during startup.

Use separate Turso databases for production and any preview environment that performs ingestion.

## 2. Fly app

`fly.toml` defines a stateless app in `iad` with internal port `8080` and a health check on `/api/v1/healthz`.

Create the Fly application if needed, then configure runtime secrets:

```bash
fly secrets set \
  TURSO_DATABASE_URL='...' \
  TURSO_AUTH_TOKEN='...' \
  ADMIN_TOKEN='...' \
  INGEST_SECRET='...'
```

Optional runtime secrets:

```text
REPORT_WEBHOOK_URL
XVIDEOS_PARTNER_JSON_ENDPOINT
XVIDEOS_PARTNER_AUTHORIZATION
```

There is deliberately no Fly volume. All durable application state belongs in Turso.

`BASE_URL` defaults to `https://plantingtulips.com` in `fly.toml`; update it if the production hostname changes.

## 3. Frontend build variables

The frontend supports:

```text
VITE_AD_MODE=off | preview | exoclick
VITE_EXOCLICK_SCRIPT_URL
VITE_EXOCLICK_ZONE_FEED_BANNER
VITE_EXOCLICK_ZONE_SIDEBAR
VITE_EXOCLICK_ZONE_PLAYER_BELOW
VITE_EXOCLICK_ZONE_NATIVE_CARD
```

`off` remains the production default until an approved publisher account is configured. If ExoClick is enabled, update the Go Content-Security-Policy to permit only the exact account-generated script/frame/connect origins required by the tag.

## 4. Production image

The Dockerfile is intentionally the same shape as the other Go/Solid applications:

1. Node 24 + pnpm installs the frozen frontend lockfile.
2. Vite builds the SolidJS 2 SPA.
3. Go 1.25 downloads the Turso modules.
4. Go tests execute against the in-memory Turso engine.
5. The Go binary is compiled with the generated `dist/` embedded.
6. A distroless non-root image contains only the resulting binary.

Build locally with:

```bash
docker build -t plantingtulips:local .
```

Deploy with the normal Fly workflow:

```bash
fly deploy
```

## 5. First catalog ingestion

After the Go origin is healthy and Turso migrations have completed:

```bash
curl -fsS \
  -X POST \
  -H "Authorization: Bearer $INGEST_SECRET" \
  -H 'Content-Type: application/json' \
  "$SITE_URL/api/v1/admin/ingest" \
  -d '{"pages":1,"syncRemoved":true}'
```

Verify that EPorner imports records and that removed-ID synchronization completes. XVideos should remain disabled unless an approved partner/feed endpoint has been configured.

## 6. Scheduled operations

GitHub Actions requires repository secrets:

```text
SITE_URL
INGEST_SECRET
```

`Catalog sync` calls:

```text
POST /api/v1/admin/ingest
POST /api/v1/admin/maintenance
```

The maintenance call prunes raw first-party analytics outside the 90-day window while keeping aggregate engagement/monetization counters.

## 7. Production verification

Preview/non-strict smoke:

```bash
SITE_URL=https://your-domain.example pnpm smoke:production
```

Production catalog gate:

```bash
SITE_URL=https://your-domain.example \
REQUIRE_CATALOG=1 \
pnpm smoke:production
```

Full growth gate:

```bash
SITE_URL=https://your-domain.example \
REQUIRE_CATALOG=1 \
REQUIRE_GROWTH=1 \
pnpm smoke:production
```

Strict smoke verifies:

- `/api/v1/healthz` confirms Go + Turso connectivity;
- the SPA is served from the Go binary;
- initial HTML has `x-pt-seo: go`;
- public category pages are indexable;
- `/admin`, `/library`, and `/report` are noindex;
- `robots.txt` advertises the production sitemap;
- catalog search is served from Turso;
- analytics events persist;
- a catalog video has detail + related results;
- its watch route includes server-rendered `VideoObject` JSON-LD and OpenGraph image metadata.

## Release gate

The old Cloudflare Pages/D1 release gate is superseded by this architecture.

Do not promote a release until:

1. Turso production credentials are configured;
2. the Fly app starts and `/api/v1/healthz` is green;
3. embedded migrations are recorded in `schema_migrations`;
4. first catalog ingestion succeeds;
5. strict production smoke passes;
6. scheduled ingestion/maintenance runs successfully;
7. provider/ad integrations stay disabled unless their account-specific setup is verified.

Cloudflare, if used, should point/proxy the public hostname to the Fly application and should not contain a second copy of application logic.
