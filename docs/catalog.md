# Catalog and ingestion

PlantingTulips keeps a persistent catalog between approved provider feeds and the public UI.

## Turso

Turso/libSQL is the catalog system of record. The Go server requires:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

Migrations in `migrations/*.sql` are embedded in the Go binary. On startup, the server creates `schema_migrations` and applies each unapplied migration in lexical order.

The schema stores provider identities, normalized metadata, tags, category membership, reports, local content blocks, provider sync state, analytics events, engagement metrics, and monetization counters.

## Ingestion

`POST /api/v1/admin/ingest` requires `Authorization: Bearer <INGEST_SECRET>` (the admin token is also accepted). A normal run ingests one page of every active category from each enabled provider and synchronizes EPorner's removed-ID list.

GitHub Actions workflow `Catalog sync` calls this endpoint daily when `SITE_URL` and `INGEST_SECRET` are configured.

Example body:

```json
{
  "pages": 1,
  "syncRemoved": true
}
```

EPorner uses its official API. XVideos stays disabled until an approved partner/feed endpoint is configured; no scraper path exists.

## Serving

`GET /api/v1/videos` queries Turso first. If the catalog is empty for a query, enabled provider adapters may supply a live fallback.

`GET /api/v1/video/{provider}/{id}` prefers Turso and can best-effort cache an approved live provider record.

`GET /api/v1/related/{provider}/{id}` ranks active, non-blocked records using normalized tag/category overlap, duration similarity, catalog score, and first-party engagement metrics.

Cross-provider duplicates are suppressed using normalized title plus duration buckets while the underlying provider rows remain available for operations.

## Moderation

`POST /api/v1/report` always persists valid reports in Turso. An optional `REPORT_WEBHOOK_URL` can receive a relay after persistence.

Reports marked `ncii` or `minor` immediately create a local safety hold for the referenced provider/video ID pending operator review.

`GET /api/v1/admin/reports` and `POST /api/v1/admin/block` require `ADMIN_TOKEN` (falling back to `INGEST_SECRET`).

## Analytics retention

Raw first-party analytics events are retained for 90 days by default. Aggregate video/ad counters remain after raw-event pruning. Scheduled catalog maintenance and the `/admin` dashboard can invoke the same retention operation.
