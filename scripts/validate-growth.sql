PRAGMA foreign_keys = ON;

CREATE TEMP TABLE assertions (
  ok INTEGER NOT NULL CHECK (ok = 1)
);

INSERT INTO assertions
SELECT CASE WHEN EXISTS (
  SELECT 1 FROM pragma_table_info('videos') WHERE name = 'canonical_key'
) THEN 1 ELSE 0 END;

INSERT INTO analytics_events(session_id, event_name, path, metadata_json, created_at) VALUES
  ('contract-session', 'video_impression', '/', '{"context":"feed"}', CURRENT_TIMESTAMP),
  ('contract-session', 'video_impression', '/', '{"context":"related"}', CURRENT_TIMESTAMP),
  ('contract-session', 'video_open', '/watch/x/y', '{}', CURRENT_TIMESTAMP),
  ('contract-session', 'related_click', '/watch/x/y', '{}', CURRENT_TIMESTAMP),
  ('contract-old', 'page_view', '/', '{}', DATETIME('now', '-120 day'));

INSERT INTO assertions
SELECT CASE WHEN (
  SELECT SUM(CASE WHEN event_name = 'video_impression' AND COALESCE(json_extract(metadata_json, '$.context'), 'feed') <> 'related' THEN 1 ELSE 0 END)
  FROM analytics_events
) = 1 THEN 1 ELSE 0 END;

INSERT INTO assertions
SELECT CASE WHEN (
  SELECT SUM(CASE WHEN event_name = 'video_impression' AND json_extract(metadata_json, '$.context') = 'related' THEN 1 ELSE 0 END)
  FROM analytics_events
) = 1 THEN 1 ELSE 0 END;

INSERT INTO assertions
SELECT CASE WHEN (
  SELECT COUNT(DISTINCT session_id) FROM analytics_events WHERE created_at >= DATETIME('now', '-7 day')
) = 1 THEN 1 ELSE 0 END;

DELETE FROM analytics_events WHERE created_at < DATETIME('now', '-90 day');
INSERT INTO assertions
SELECT CASE WHEN NOT EXISTS (
  SELECT 1 FROM analytics_events WHERE session_id = 'contract-old'
) THEN 1 ELSE 0 END;

INSERT INTO video_metrics(provider, provider_id, impressions, opens, related_clicks, source_clicks, favorites, hides, last_event_at)
VALUES ('eporner', 'contract-video', 2, 1, 1, 1, 1, 0, CURRENT_TIMESTAMP)
ON CONFLICT(provider, provider_id) DO UPDATE SET opens = opens + 1;

INSERT INTO assertions
SELECT CASE WHEN EXISTS (
  SELECT 1 FROM video_metrics WHERE provider = 'eporner' AND provider_id = 'contract-video' AND opens >= 1
) THEN 1 ELSE 0 END;

INSERT INTO monetization_metrics(day, slot, provider, impressions, clicks)
VALUES (DATE('now'), 'contract-slot', 'preview', 1, 1)
ON CONFLICT(day, slot, provider) DO UPDATE SET impressions = impressions + 1, clicks = clicks + 1;

INSERT INTO assertions
SELECT CASE WHEN EXISTS (
  SELECT 1 FROM monetization_metrics WHERE day = DATE('now') AND slot = 'contract-slot' AND impressions >= 1 AND clicks >= 1
) THEN 1 ELSE 0 END;

SELECT 'growth database contract ok' AS result;
