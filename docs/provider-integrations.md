# Provider integrations

## Policy

Only integrate APIs, feeds, or embeds the provider authorizes for third-party publishing. Do not build production dependencies on HTML scraping, extracted CDN URLs, or downloader tooling.

## Eporner

Official API base:

`https://www.eporner.com/api/v2/video/`

Used methods:

- `search`
- `id`
- eventually `removed` for tombstone synchronization

The current Pages Function requests JSON and normalizes Eporner fields to the app's `VideoSummary` model.

## XVideos

XVideos has offered webmaster/affiliate database and feed products, but the current access method is partner-specific. The repository therefore includes the provider boundary without pretending there is a stable anonymous JSON search API.

When official access is approved:

1. Receive the provider's authorized CSV/XML/database feed.
2. Normalize it in an operator-controlled ingestion job or endpoint.
3. Expose objects shaped like:

```json
{
  "videos": [
    {
      "id": "provider-id",
      "title": "title",
      "thumbnailUrl": "https://...",
      "duration": "12:34",
      "views": 1234,
      "rating": 96,
      "sourceUrl": "https://...",
      "embedUrl": "https://...",
      "tags": ["..."]
    }
  ]
}
```

4. Configure `XVIDEOS_PARTNER_JSON_ENDPOINT` in Cloudflare Pages.
5. Add removed/tombstone synchronization before treating the feed as durable catalog data.

## Next provider work

- provider-level retries/timeouts
- deduplication across providers
- normalized tags/taxonomy
- removed-video sync
- local moderation denylist
- provider health metrics
