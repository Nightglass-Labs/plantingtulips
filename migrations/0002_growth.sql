PRAGMA foreign_keys = ON;

ALTER TABLE videos ADD COLUMN canonical_key TEXT;
CREATE INDEX IF NOT EXISTS idx_videos_canonical_key ON videos(canonical_key, active, ranking_score DESC);

CREATE TABLE IF NOT EXISTS analytics_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  event_name TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  provider TEXT,
  provider_id TEXT,
  category TEXT,
  query TEXT,
  slot TEXT,
  value REAL,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_name_created ON analytics_events(event_name, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_session_created ON analytics_events(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_video ON analytics_events(provider, provider_id, event_name, created_at DESC);

CREATE TABLE IF NOT EXISTS video_metrics (
  provider TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  impressions INTEGER NOT NULL DEFAULT 0,
  opens INTEGER NOT NULL DEFAULT 0,
  related_clicks INTEGER NOT NULL DEFAULT 0,
  source_clicks INTEGER NOT NULL DEFAULT 0,
  favorites INTEGER NOT NULL DEFAULT 0,
  hides INTEGER NOT NULL DEFAULT 0,
  last_event_at TEXT,
  PRIMARY KEY (provider, provider_id)
);

CREATE TABLE IF NOT EXISTS monetization_metrics (
  day TEXT NOT NULL,
  slot TEXT NOT NULL,
  provider TEXT NOT NULL,
  impressions INTEGER NOT NULL DEFAULT 0,
  clicks INTEGER NOT NULL DEFAULT 0,
  revenue REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (day, slot, provider)
);

CREATE TABLE IF NOT EXISTS operator_notes (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
