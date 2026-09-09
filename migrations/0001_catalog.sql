PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS providers (
  name TEXT PRIMARY KEY,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_ok_at TEXT,
  last_error_at TEXT,
  last_error TEXT
);

CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  query TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS category_aliases (
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  alias TEXT NOT NULL UNIQUE,
  PRIMARY KEY (category_id, alias)
);

CREATE TABLE IF NOT EXISTS videos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  provider TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  title TEXT NOT NULL,
  thumbnail_url TEXT NOT NULL,
  duration TEXT NOT NULL DEFAULT '',
  duration_seconds INTEGER NOT NULL DEFAULT 0,
  views INTEGER,
  rating REAL,
  source_url TEXT NOT NULL,
  embed_url TEXT NOT NULL,
  tags_json TEXT NOT NULL DEFAULT '[]',
  published_at TEXT,
  imported_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ranking_score REAL NOT NULL DEFAULT 0,
  active INTEGER NOT NULL DEFAULT 1,
  UNIQUE(provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_videos_active_rank ON videos(active, ranking_score DESC);
CREATE INDEX IF NOT EXISTS idx_videos_active_imported ON videos(active, imported_at DESC);
CREATE INDEX IF NOT EXISTS idx_videos_provider_id ON videos(provider, provider_id);

CREATE TABLE IF NOT EXISTS tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS video_tags (
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (video_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_video_tags_tag ON video_tags(tag_id, video_id);

CREATE TABLE IF NOT EXISTS video_categories (
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  source TEXT NOT NULL DEFAULT 'ingest',
  confidence REAL NOT NULL DEFAULT 1,
  PRIMARY KEY (video_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_video_categories_category ON video_categories(category_id, video_id);

CREATE TABLE IF NOT EXISTS reports (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  reason TEXT NOT NULL,
  provider TEXT,
  provider_id TEXT,
  source_url TEXT,
  details TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reviewed_at TEXT,
  resolution TEXT
);

CREATE INDEX IF NOT EXISTS idx_reports_status_created ON reports(status, created_at DESC);

CREATE TABLE IF NOT EXISTS blocked_content (
  provider TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  report_id TEXT REFERENCES reports(id) ON DELETE SET NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (provider, provider_id)
);

CREATE TABLE IF NOT EXISTS provider_sync_state (
  provider TEXT PRIMARY KEY,
  last_sync_at TEXT,
  last_removed_sync_at TEXT,
  last_error TEXT,
  imported_count INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO providers(name, enabled) VALUES ('eporner', 1), ('xvideos', 0);

INSERT OR IGNORE INTO categories(slug, name, description, query, sort_order) VALUES
  ('amateur', 'Amateur', 'Community-style and independently produced results.', 'amateur blowjob', 10),
  ('pov', 'POV', 'Point-of-view focused browsing.', 'POV blowjob', 20),
  ('deepthroat', 'Deepthroat', 'A dedicated focused subcategory.', 'deepthroat', 30),
  ('compilations', 'Compilations', 'Compilation-focused browsing.', 'blowjob compilation', 40),
  ('long-form', 'Long-form', 'Longer videos grouped into one durable hub.', 'long blowjob', 50),
  ('short', 'Short', 'Short-form videos for quick browsing.', 'short blowjob', 60);

INSERT OR IGNORE INTO category_aliases(category_id, alias)
SELECT id, 'amateur blowjob' FROM categories WHERE slug = 'amateur';
INSERT OR IGNORE INTO category_aliases(category_id, alias)
SELECT id, 'point of view' FROM categories WHERE slug = 'pov';
INSERT OR IGNORE INTO category_aliases(category_id, alias)
SELECT id, 'compilation' FROM categories WHERE slug = 'compilations';
