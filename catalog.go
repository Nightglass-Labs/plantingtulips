package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

type CategorySummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Query       string `json:"query"`
	VideoCount  int64  `json:"videoCount"`
}

type ProviderStatus struct {
	Enabled bool   `json:"enabled"`
	Note    string `json:"note,omitempty"`
}

type videoRow struct {
	Provider     ProviderName
	ProviderID   string
	Title        string
	ThumbnailURL string
	Duration     string
	DurationSecs int
	Views        sql.NullInt64
	Rating       sql.NullFloat64
	SourceURL    string
	EmbedURL     string
	TagsJSON     string
}

type Report struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Reason    string `json:"reason"`
	Provider  string `json:"provider,omitempty"`
	VideoID   string `json:"videoId,omitempty"`
	SourceURL string `json:"sourceUrl,omitempty"`
	Details   string `json:"details"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func (a *app) searchCatalog(ctx context.Context, query string, page int, sort string) (providerSearchResult, error) {
	const limit = 48
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	q := strings.ToLower(strings.TrimSpace(query))
	filter := ""
	filterArgs := []any{}
	if q != "" && q != "all" {
		filter = " AND (LOWER(v.title) LIKE ? OR LOWER(v.tags_json) LIKE ?)"
		filterArgs = append(filterArgs, "%"+q+"%", "%"+q+"%")
	}
	dedupe := `
    AND (
      v.canonical_key IS NULL OR v.canonical_key = '' OR v.id = (
        SELECT MIN(d.id) FROM videos d
        WHERE d.active = 1 AND d.canonical_key = v.canonical_key
      )
    )`
	querySQL := `
SELECT v.provider, v.provider_id, v.title, v.thumbnail_url, v.duration, v.duration_seconds,
       v.views, v.rating, v.source_url, v.embed_url, v.tags_json
FROM videos v
LEFT JOIN video_metrics vm ON vm.provider = v.provider AND vm.provider_id = v.provider_id
WHERE v.active = 1
  AND NOT EXISTS (
    SELECT 1 FROM blocked_content b
    WHERE b.provider = v.provider AND b.provider_id = v.provider_id
  )` + dedupe + filter + `
ORDER BY ` + orderBy(sort) + `
LIMIT ? OFFSET ?`
	args := append(append([]any{}, filterArgs...), limit, offset)
	rows, err := a.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return providerSearchResult{}, err
	}
	videos, err := scanVideos(rows)
	if err != nil {
		return providerSearchResult{}, err
	}

	countSQL := `
SELECT COUNT(*)
FROM videos v
WHERE v.active = 1
  AND NOT EXISTS (
    SELECT 1 FROM blocked_content b
    WHERE b.provider = v.provider AND b.provider_id = v.provider_id
  )` + dedupe + filter
	var total int
	if err := a.db.QueryRowContext(ctx, countSQL, filterArgs...).Scan(&total); err != nil {
		return providerSearchResult{}, err
	}
	return providerSearchResult{
		Page:       page,
		TotalPages: max(1, int(math.Ceil(float64(total)/limit))),
		TotalCount: total,
		Videos:     videos,
	}, nil
}

func (a *app) getCatalogVideo(ctx context.Context, provider, providerID string) (VideoSummary, bool, error) {
	row := a.db.QueryRowContext(ctx, `
SELECT v.provider, v.provider_id, v.title, v.thumbnail_url, v.duration, v.duration_seconds,
       v.views, v.rating, v.source_url, v.embed_url, v.tags_json
FROM videos v
WHERE v.provider = ? AND v.provider_id = ? AND v.active = 1
  AND NOT EXISTS (
    SELECT 1 FROM blocked_content b
    WHERE b.provider = v.provider AND b.provider_id = v.provider_id
  )
LIMIT 1`, provider, providerID)
	videoRow, err := scanVideoRow(row)
	if err == sql.ErrNoRows {
		return VideoSummary{}, false, nil
	}
	if err != nil {
		return VideoSummary{}, false, err
	}
	return rowToVideo(videoRow), true, nil
}

func (a *app) isBlocked(ctx context.Context, provider, providerID string) (bool, error) {
	var value int
	err := a.db.QueryRowContext(ctx, `SELECT 1 FROM blocked_content WHERE provider = ? AND provider_id = ? LIMIT 1`, provider, providerID).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil && value == 1, err
}

func (a *app) upsertVideo(ctx context.Context, video VideoSummary, categorySlug string) error {
	tags := normalizeTags(video.Tags)
	durationSeconds := parseDuration(video.Duration)
	rankingScore := rankVideo(video.Views, video.Rating)
	canonicalKey := canonicalize(video.Title, durationSeconds)

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
INSERT INTO videos(
  provider, provider_id, title, thumbnail_url, duration, duration_seconds,
  views, rating, source_url, embed_url, tags_json, ranking_score, canonical_key, active, last_seen_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
ON CONFLICT(provider, provider_id) DO UPDATE SET
  title = excluded.title,
  thumbnail_url = excluded.thumbnail_url,
  duration = excluded.duration,
  duration_seconds = excluded.duration_seconds,
  views = excluded.views,
  rating = excluded.rating,
  source_url = excluded.source_url,
  embed_url = excluded.embed_url,
  tags_json = excluded.tags_json,
  ranking_score = excluded.ranking_score,
  canonical_key = excluded.canonical_key,
  active = 1,
  last_seen_at = CURRENT_TIMESTAMP`,
		video.Provider, video.ID, video.Title, video.ThumbnailURL, video.Duration, durationSeconds,
		nullInt(video.Views), nullFloat(video.Rating), video.SourceURL, video.EmbedURL, mustJSON(tagNames(tags)), rankingScore, canonicalKey,
	)
	if err != nil {
		return err
	}
	var videoID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM videos WHERE provider = ? AND provider_id = ?`, video.Provider, video.ID).Scan(&videoID); err != nil {
		return err
	}
	if categorySlug != "" {
		if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO video_categories(video_id, category_id, source, confidence)
SELECT ?, id, 'ingest', 1 FROM categories WHERE slug = ? AND active = 1`, videoID, categorySlug); err != nil {
			return err
		}
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO tags(slug, name) VALUES (?, ?)`, tag.Slug, tag.Name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO video_tags(video_id, tag_id)
SELECT ?, id FROM tags WHERE slug = ?`, videoID, tag.Slug); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *app) relatedVideos(ctx context.Context, provider, providerID string, limit int) ([]VideoSummary, error) {
	var sourceID, durationSeconds int
	var canonicalKey sql.NullString
	if err := a.db.QueryRowContext(ctx, `SELECT id, duration_seconds, canonical_key FROM videos WHERE provider = ? AND provider_id = ?`, provider, providerID).Scan(&sourceID, &durationSeconds, &canonicalKey); err != nil {
		if err == sql.ErrNoRows {
			return []VideoSummary{}, nil
		}
		return nil, err
	}
	limit = max(1, min(24, limit))
	rows, err := a.db.QueryContext(ctx, `
WITH candidate_overlap AS (
  SELECT candidate_tags.video_id AS video_id, COUNT(*) AS tag_overlap, 0 AS category_overlap
  FROM video_tags source_tags
  JOIN video_tags candidate_tags ON candidate_tags.tag_id = source_tags.tag_id
  WHERE source_tags.video_id = ? AND candidate_tags.video_id <> ?
  GROUP BY candidate_tags.video_id
  UNION ALL
  SELECT candidate_categories.video_id AS video_id, 0 AS tag_overlap, COUNT(*) AS category_overlap
  FROM video_categories source_categories
  JOIN video_categories candidate_categories ON candidate_categories.category_id = source_categories.category_id
  WHERE source_categories.video_id = ? AND candidate_categories.video_id <> ?
  GROUP BY candidate_categories.video_id
), overlaps AS (
  SELECT video_id, SUM(tag_overlap) AS tag_overlap, SUM(category_overlap) AS category_overlap
  FROM candidate_overlap
  GROUP BY video_id
)
SELECT v.provider, v.provider_id, v.title, v.thumbnail_url, v.duration, v.duration_seconds,
       v.views, v.rating, v.source_url, v.embed_url, v.tags_json
FROM overlaps o
JOIN videos v ON v.id = o.video_id
LEFT JOIN video_metrics vm ON vm.provider = v.provider AND vm.provider_id = v.provider_id
WHERE v.active = 1
  AND (? = '' OR v.canonical_key IS NULL OR v.canonical_key <> ?)
  AND NOT EXISTS (
    SELECT 1 FROM blocked_content b
    WHERE b.provider = v.provider AND b.provider_id = v.provider_id
  )
ORDER BY (
  o.tag_overlap * 25 +
  o.category_overlap * 35 +
  v.ranking_score * 0.15 +
  COALESCE(vm.opens, 0) * 0.8 +
  COALESCE(vm.related_clicks, 0) * 2.0 +
  COALESCE(vm.favorites, 0) * 1.5 -
  COALESCE(vm.hides, 0) * 4.0 -
  MIN(20, ABS(v.duration_seconds - ?) / 60.0)
) DESC, v.imported_at DESC
LIMIT ?`, sourceID, sourceID, sourceID, sourceID, canonicalKey.String, canonicalKey.String, durationSeconds, limit)
	if err != nil {
		return nil, err
	}
	return scanVideos(rows)
}

func (a *app) listCategories(ctx context.Context) ([]CategorySummary, error) {
	rows, err := a.db.QueryContext(ctx, `
SELECT c.slug, c.name, c.description, c.query,
  (
    SELECT COUNT(*)
    FROM video_categories vc
    JOIN videos v ON v.id = vc.video_id
    WHERE vc.category_id = c.id
      AND v.active = 1
      AND NOT EXISTS (
        SELECT 1 FROM blocked_content b
        WHERE b.provider = v.provider AND b.provider_id = v.provider_id
      )
  ) AS video_count
FROM categories c
WHERE c.active = 1
ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CategorySummary{}
	for rows.Next() {
		var item CategorySummary
		if err := rows.Scan(&item.Slug, &item.Name, &item.Description, &item.Query, &item.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *app) deactivateProviderIDs(ctx context.Context, provider ProviderName, ids []string) error {
	for _, id := range ids {
		if _, err := a.db.ExecContext(ctx, `UPDATE videos SET active = 0 WHERE provider = ? AND provider_id = ?`, provider, id); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) providerEnabled(ctx context.Context, provider ProviderName) (bool, error) {
	var enabled int
	err := a.db.QueryRowContext(ctx, `SELECT enabled FROM providers WHERE name = ?`, provider).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return enabled != 0, err
}

func (a *app) providerStatuses(ctx context.Context) (map[ProviderName]ProviderStatus, error) {
	out := map[ProviderName]ProviderStatus{
		ProviderEporner: {Enabled: false},
		ProviderXvideos: {Enabled: false},
	}
	rows, err := a.db.QueryContext(ctx, `SELECT name, enabled, COALESCE(last_error, '') FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name ProviderName
		var enabled int
		var note string
		if err := rows.Scan(&name, &enabled, &note); err != nil {
			return nil, err
		}
		if name == ProviderXvideos && a.xvideosEndpoint == "" && note == "" {
			note = "approved partner feed is not configured"
		}
		out[name] = ProviderStatus{Enabled: enabled != 0, Note: note}
	}
	return out, rows.Err()
}

func (a *app) setProviderEnabled(ctx context.Context, provider ProviderName, enabled bool) error {
	if provider != ProviderEporner && provider != ProviderXvideos {
		return fmt.Errorf("unsupported provider")
	}
	value := 0
	if enabled {
		value = 1
	}
	_, err := a.db.ExecContext(ctx, `
INSERT INTO providers(name, enabled) VALUES (?, ?)
ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled`, provider, value)
	return err
}

func (a *app) recordProviderHealth(ctx context.Context, provider ProviderName, ok bool, message string) error {
	defaultEnabled := 1
	if provider == ProviderXvideos {
		defaultEnabled = 0
	}
	if ok {
		_, err := a.db.ExecContext(ctx, `
INSERT INTO providers(name, enabled, last_ok_at, last_error)
VALUES (?, ?, CURRENT_TIMESTAMP, NULL)
ON CONFLICT(name) DO UPDATE SET last_ok_at = CURRENT_TIMESTAMP, last_error = NULL`, provider, defaultEnabled)
		return err
	}
	_, err := a.db.ExecContext(ctx, `
INSERT INTO providers(name, enabled, last_error_at, last_error)
VALUES (?, ?, CURRENT_TIMESTAMP, ?)
ON CONFLICT(name) DO UPDATE SET last_error_at = CURRENT_TIMESTAMP, last_error = excluded.last_error`, provider, defaultEnabled, truncate(message, 1000))
	return err
}

func (a *app) markSync(ctx context.Context, provider ProviderName, imported int, removedSync bool, message string) error {
	removed := 0
	if removedSync {
		removed = 1
	}
	_, err := a.db.ExecContext(ctx, `
INSERT INTO provider_sync_state(provider, last_sync_at, last_removed_sync_at, last_error, imported_count)
VALUES (?, CURRENT_TIMESTAMP, CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP END, ?, ?)
ON CONFLICT(provider) DO UPDATE SET
  last_sync_at = CURRENT_TIMESTAMP,
  last_removed_sync_at = CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP ELSE provider_sync_state.last_removed_sync_at END,
  last_error = excluded.last_error,
  imported_count = provider_sync_state.imported_count + excluded.imported_count`, provider, removed, nullableString(message), imported, removed)
	return err
}

func (a *app) saveReport(ctx context.Context, report Report) error {
	_, err := a.db.ExecContext(ctx, `
INSERT INTO reports(id, email, reason, provider, provider_id, source_url, details)
VALUES (?, ?, ?, ?, ?, ?, ?)`, report.ID, report.Email, report.Reason, nullableString(report.Provider), nullableString(report.VideoID), nullableString(report.SourceURL), report.Details)
	return err
}

func (a *app) blockContent(ctx context.Context, provider, providerID, reason, reportID string) error {
	_, err := a.db.ExecContext(ctx, `
INSERT INTO blocked_content(provider, provider_id, reason, report_id)
VALUES (?, ?, ?, ?)
ON CONFLICT(provider, provider_id) DO UPDATE SET
  reason = excluded.reason,
  report_id = COALESCE(excluded.report_id, blocked_content.report_id)`, provider, providerID, reason, nullableString(reportID))
	return err
}

func (a *app) listReports(ctx context.Context, limit int) ([]Report, error) {
	limit = max(1, min(250, limit))
	rows, err := a.db.QueryContext(ctx, `
SELECT id, email, reason, COALESCE(provider,''), COALESCE(provider_id,''), COALESCE(source_url,''),
       details, status, created_at
FROM reports
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		var item Report
		if err := rows.Scan(&item.ID, &item.Email, &item.Reason, &item.Provider, &item.VideoID, &item.SourceURL, &item.Details, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanVideos(rows *sql.Rows) ([]VideoSummary, error) {
	defer rows.Close()
	out := []VideoSummary{}
	for rows.Next() {
		row, err := scanVideoScanner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rowToVideo(row))
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanVideoRow(row *sql.Row) (videoRow, error) { return scanVideoScanner(row) }

func scanVideoScanner(row scanner) (videoRow, error) {
	var item videoRow
	err := row.Scan(&item.Provider, &item.ProviderID, &item.Title, &item.ThumbnailURL, &item.Duration, &item.DurationSecs, &item.Views, &item.Rating, &item.SourceURL, &item.EmbedURL, &item.TagsJSON)
	return item, err
}

func rowToVideo(row videoRow) VideoSummary {
	tags := []string{}
	_ = json.Unmarshal([]byte(row.TagsJSON), &tags)
	video := VideoSummary{ID: row.ProviderID, Provider: row.Provider, Title: row.Title, ThumbnailURL: row.ThumbnailURL, Duration: row.Duration, SourceURL: row.SourceURL, EmbedURL: row.EmbedURL, Tags: tags}
	if row.Views.Valid {
		value := row.Views.Int64
		video.Views = &value
	}
	if row.Rating.Valid {
		value := row.Rating.Float64
		video.Rating = &value
	}
	return video
}

func orderBy(sort string) string {
	engagement := `(v.ranking_score + COALESCE(vm.opens, 0) * 0.8 + COALESCE(vm.related_clicks, 0) * 1.8 + COALESCE(vm.favorites, 0) * 1.2 - COALESCE(vm.hides, 0) * 3.0)`
	switch sort {
	case "latest":
		return "v.imported_at DESC"
	case "most-popular":
		return "COALESCE(v.views, 0) DESC, " + engagement + " DESC"
	case "top-rated":
		return "COALESCE(v.rating, 0) DESC, " + engagement + " DESC"
	case "longest":
		return "v.duration_seconds DESC, " + engagement + " DESC"
	case "shortest":
		return "v.duration_seconds ASC, " + engagement + " DESC"
	default:
		return engagement + " DESC, v.imported_at DESC"
	}
}

func parseDuration(value string) int {
	parts := strings.Split(value, ":")
	if len(parts) == 0 {
		return 0
	}
	seconds := 0
	for _, part := range parts {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &n); err != nil {
			return 0
		}
		seconds = seconds*60 + n
	}
	return seconds
}

func rankVideo(views *int64, rating *float64) float64 {
	var v float64
	if views != nil {
		v = float64(*views)
	}
	var r float64
	if rating != nil {
		r = *rating
	}
	return math.Log10(math.Max(0, v)+1)*20 + math.Max(0, r)*0.55
}

func canonicalize(title string, durationSeconds int) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	normalized := strings.Trim(re.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(normalized) > 140 {
		normalized = normalized[:140]
	}
	if normalized == "" {
		return ""
	}
	bucket := 0
	if durationSeconds > 0 {
		bucket = int(math.Round(float64(durationSeconds)/5.0)) * 5
	}
	return fmt.Sprintf("%s:%d", normalized, bucket)
}

type normalizedTag struct{ Slug, Name string }

func normalizeTags(values []string) []normalizedTag {
	seen := map[string]bool{}
	out := []normalizedTag{}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	for _, raw := range values {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len(name) > 100 {
			name = name[:100]
		}
		slug := strings.Trim(re.ReplaceAllString(strings.ToLower(name), "-"), "-")
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, normalizedTag{Slug: slug, Name: name})
		if len(out) == 20 {
			break
		}
	}
	return out
}

func tagNames(tags []normalizedTag) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, tag.Name)
	}
	return out
}

func mustJSON(value any) string {
	body, _ := json.Marshal(value)
	return string(body)
}

func nullInt(value *int64) any {
	if value == nil { return nil }
	return *value
}
func nullFloat(value *float64) any {
	if value == nil { return nil }
	return *value
}
func nullableString(value string) any {
	if strings.TrimSpace(value) == "" { return nil }
	return value
}
func truncate(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) > maxLen { return value[:maxLen] }
	return value
}
