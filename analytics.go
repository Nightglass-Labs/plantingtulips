package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var analyticsSessionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,96}$`)

var analyticsEvents = map[string]bool{
	"page_view": true, "search": true, "category_view": true,
	"video_impression": true, "video_open": true, "related_click": true,
	"source_click": true, "favorite": true, "unfavorite": true, "hide": true,
	"ad_impression": true, "ad_click": true,
}

type AnalyticsInput struct {
	SessionID string         `json:"sessionId"`
	EventName string         `json:"eventName"`
	Path      string         `json:"path,omitempty"`
	Provider  string         `json:"provider,omitempty"`
	VideoID   string         `json:"videoId,omitempty"`
	Category  string         `json:"category,omitempty"`
	Query     string         `json:"query,omitempty"`
	Slot      string         `json:"slot,omitempty"`
	Value     *float64       `json:"value,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type namedCount struct {
	Status   string `json:"status,omitempty"`
	Event    string `json:"event_name,omitempty"`
	Query    string `json:"query,omitempty"`
	Category string `json:"category,omitempty"`
	Count    int64  `json:"count"`
}

type providerAdminRow struct {
	Name        ProviderName `json:"name"`
	Enabled     int          `json:"enabled"`
	LastOKAt    *string      `json:"lastOkAt,omitempty"`
	LastErrorAt *string      `json:"lastErrorAt,omitempty"`
	LastError   *string      `json:"lastError,omitempty"`
}

type syncAdminRow struct {
	Provider          ProviderName `json:"provider"`
	LastSyncAt        *string      `json:"lastSyncAt,omitempty"`
	LastRemovedSyncAt *string      `json:"lastRemovedSyncAt,omitempty"`
	LastError         *string      `json:"lastError,omitempty"`
	ImportedCount     int64        `json:"importedCount"`
}

type topVideoMetric struct {
	Provider      ProviderName `json:"provider"`
	VideoID       string       `json:"videoId"`
	Opens         int64        `json:"opens"`
	RelatedClicks int64        `json:"relatedClicks"`
	SourceClicks  int64        `json:"sourceClicks"`
	Favorites     int64        `json:"favorites"`
	Hides         int64        `json:"hides"`
}

type adMetric struct {
	Day         string  `json:"day"`
	Slot        string  `json:"slot"`
	Provider    string  `json:"provider"`
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	Revenue     float64 `json:"revenue"`
}

type funnelMetrics struct {
	Sessions           int64 `json:"sessions"`
	Searches           int64 `json:"searches"`
	FeedImpressions    int64 `json:"feedImpressions"`
	RelatedImpressions int64 `json:"relatedImpressions"`
	VideoOpens         int64 `json:"videoOpens"`
	RelatedClicks      int64 `json:"relatedClicks"`
	SourceClicks       int64 `json:"sourceClicks"`
	Favorites          int64 `json:"favorites"`
}

type retentionMetrics struct {
	RawEvents      int64   `json:"rawEvents"`
	OldestEvent    *string `json:"oldestEvent"`
	NewestEvent    *string `json:"newestEvent"`
	ConfiguredDays int     `json:"configuredDays"`
}

type adminDashboardResponse struct {
	Catalog struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"catalog"`
	Reports            []namedCount       `json:"reports"`
	EventsToday        []namedCount       `json:"eventsToday"`
	Providers          []providerAdminRow `json:"providers"`
	Sync               []syncAdminRow     `json:"sync"`
	TopSearches        []namedCount       `json:"topSearches"`
	TopCategories      []namedCount       `json:"topCategories"`
	TopVideos          []topVideoMetric   `json:"topVideos"`
	Ads                []adMetric         `json:"ads"`
	Funnel7d           funnelMetrics      `json:"funnel7d"`
	AnalyticsRetention retentionMetrics   `json:"analyticsRetention"`
}

func (a *app) analyticsRateLimited(ctx context.Context, sessionID string, limit int) (bool, error) {
	sessionID = truncate(sessionID, 96)
	if !analyticsSessionPattern.MatchString(sessionID) {
		return true, nil
	}
	limit = max(10, min(300, limit))
	var count int
	if err := a.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM analytics_events
WHERE session_id = ? AND created_at >= DATETIME('now', '-1 minute')`, sessionID).Scan(&count); err != nil {
		return false, err
	}
	return count >= limit, nil
}

func (a *app) recordAnalyticsEvent(ctx context.Context, input AnalyticsInput) error {
	input.SessionID = truncate(input.SessionID, 96)
	input.EventName = truncate(input.EventName, 64)
	if !analyticsSessionPattern.MatchString(input.SessionID) || !analyticsEvents[input.EventName] {
		return fmt.Errorf("invalid analytics event")
	}
	provider := truncate(input.Provider, 32)
	videoID := truncate(input.VideoID, 160)
	slot := truncate(input.Slot, 80)
	metadata := sanitizeMetadata(input.Metadata)
	metadataJSON, _ := json.Marshal(metadata)
	_, err := a.db.ExecContext(ctx, `
INSERT INTO analytics_events(
  session_id, event_name, path, provider, provider_id, category, query, slot, value, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.SessionID,
		input.EventName,
		truncate(input.Path, 320),
		nullableString(provider),
		nullableString(videoID),
		nullableString(truncate(input.Category, 100)),
		nullableString(truncate(input.Query, 160)),
		nullableString(slot),
		nullFloat(input.Value),
		string(metadataJSON),
	)
	if err != nil {
		return err
	}

	if provider != "" && videoID != "" {
		if column := metricColumn(input.EventName); column != "" {
			query := fmt.Sprintf(`
INSERT INTO video_metrics(provider, provider_id, %s, last_event_at)
VALUES (?, ?, 1, CURRENT_TIMESTAMP)
ON CONFLICT(provider, provider_id) DO UPDATE SET
  %s = %s + 1,
  last_event_at = CURRENT_TIMESTAMP`, column, column, column)
			if _, err := a.db.ExecContext(ctx, query, provider, videoID); err != nil {
				return err
			}
		}
	}

	if slot != "" && (input.EventName == "ad_impression" || input.EventName == "ad_click") {
		column := "impressions"
		if input.EventName == "ad_click" {
			column = "clicks"
		}
		adProvider := "unknown"
		if value, ok := metadata["adProvider"].(string); ok && strings.TrimSpace(value) != "" {
			adProvider = truncate(value, 64)
		}
		query := fmt.Sprintf(`
INSERT INTO monetization_metrics(day, slot, provider, %s)
VALUES (DATE('now'), ?, ?, 1)
ON CONFLICT(day, slot, provider) DO UPDATE SET %s = %s + 1`, column, column, column)
		if _, err := a.db.ExecContext(ctx, query, slot, adProvider); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) pruneAnalytics(ctx context.Context, retentionDays int) (map[string]any, error) {
	retentionDays = max(7, min(365, retentionDays))
	var before, after int64
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analytics_events`).Scan(&before); err != nil {
		return nil, err
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM analytics_events WHERE created_at < DATETIME('now', ?)`, fmt.Sprintf("-%d day", retentionDays)); err != nil {
		return nil, err
	}
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analytics_events`).Scan(&after); err != nil {
		return nil, err
	}
	return map[string]any{
		"retentionDays": retentionDays,
		"deletedEvents": max(int64(0), before-after),
		"remainingEvents": after,
	}, nil
}

func (a *app) adminDashboard(ctx context.Context) (adminDashboardResponse, error) {
	var out adminDashboardResponse
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN active = 1 THEN 1 ELSE 0 END),0) FROM videos`).Scan(&out.Catalog.Total, &out.Catalog.Active); err != nil {
		return out, err
	}
	var err error
	if out.Reports, err = queryNamedCounts(ctx, a.db, `SELECT status, COUNT(*) FROM reports GROUP BY status`, "status"); err != nil { return out, err }
	if out.EventsToday, err = queryNamedCounts(ctx, a.db, `SELECT event_name, COUNT(*) FROM analytics_events WHERE created_at >= DATETIME('now', '-1 day') GROUP BY event_name ORDER BY COUNT(*) DESC`, "event"); err != nil { return out, err }
	if out.TopSearches, err = queryNamedCounts(ctx, a.db, `SELECT query, COUNT(*) FROM analytics_events WHERE event_name='search' AND query IS NOT NULL AND created_at >= DATETIME('now', '-30 day') GROUP BY query ORDER BY COUNT(*) DESC LIMIT 20`, "query"); err != nil { return out, err }
	if out.TopCategories, err = queryNamedCounts(ctx, a.db, `SELECT category, COUNT(*) FROM analytics_events WHERE event_name='category_view' AND category IS NOT NULL AND created_at >= DATETIME('now', '-30 day') GROUP BY category ORDER BY COUNT(*) DESC LIMIT 20`, "category"); err != nil { return out, err }
	if out.Providers, err = a.queryAdminProviders(ctx); err != nil { return out, err }
	if out.Sync, err = a.queryAdminSync(ctx); err != nil { return out, err }
	if out.TopVideos, err = a.queryTopVideos(ctx); err != nil { return out, err }
	if out.Ads, err = a.queryAds(ctx); err != nil { return out, err }

	if err := a.db.QueryRowContext(ctx, `
SELECT
  COUNT(DISTINCT session_id),
  COALESCE(SUM(CASE WHEN event_name = 'search' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'video_impression' AND COALESCE(json_extract(metadata_json, '$.context'), 'feed') <> 'related' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'video_impression' AND json_extract(metadata_json, '$.context') = 'related' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'video_open' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'related_click' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'source_click' THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN event_name = 'favorite' THEN 1 ELSE 0 END),0)
FROM analytics_events
WHERE created_at >= DATETIME('now', '-7 day')`).Scan(
		&out.Funnel7d.Sessions, &out.Funnel7d.Searches, &out.Funnel7d.FeedImpressions,
		&out.Funnel7d.RelatedImpressions, &out.Funnel7d.VideoOpens, &out.Funnel7d.RelatedClicks,
		&out.Funnel7d.SourceClicks, &out.Funnel7d.Favorites,
	); err != nil { return out, err }

	var oldest, newest sql.NullString
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at), MAX(created_at) FROM analytics_events`).Scan(&out.AnalyticsRetention.RawEvents, &oldest, &newest); err != nil { return out, err }
	out.AnalyticsRetention.ConfiguredDays = 90
	if oldest.Valid { value := oldest.String; out.AnalyticsRetention.OldestEvent = &value }
	if newest.Valid { value := newest.String; out.AnalyticsRetention.NewestEvent = &value }
	return out, nil
}

func queryNamedCounts(ctx context.Context, db *sql.DB, query, field string) ([]namedCount, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []namedCount{}
	for rows.Next() {
		var value string
		var count int64
		if err := rows.Scan(&value, &count); err != nil { return nil, err }
		item := namedCount{Count: count}
		switch field {
		case "status": item.Status = value
		case "event": item.Event = value
		case "query": item.Query = value
		case "category": item.Category = value
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *app) queryAdminProviders(ctx context.Context) ([]providerAdminRow, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT name, enabled, last_ok_at, last_error_at, last_error FROM providers ORDER BY name`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []providerAdminRow{}
	for rows.Next() {
		var item providerAdminRow
		var okAt, errorAt, lastError sql.NullString
		if err := rows.Scan(&item.Name, &item.Enabled, &okAt, &errorAt, &lastError); err != nil { return nil, err }
		if okAt.Valid { v := okAt.String; item.LastOKAt = &v }
		if errorAt.Valid { v := errorAt.String; item.LastErrorAt = &v }
		if lastError.Valid { v := lastError.String; item.LastError = &v }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *app) queryAdminSync(ctx context.Context) ([]syncAdminRow, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT provider, last_sync_at, last_removed_sync_at, last_error, imported_count FROM provider_sync_state ORDER BY provider`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []syncAdminRow{}
	for rows.Next() {
		var item syncAdminRow
		var syncAt, removedAt, lastError sql.NullString
		if err := rows.Scan(&item.Provider, &syncAt, &removedAt, &lastError, &item.ImportedCount); err != nil { return nil, err }
		if syncAt.Valid { v := syncAt.String; item.LastSyncAt = &v }
		if removedAt.Valid { v := removedAt.String; item.LastRemovedSyncAt = &v }
		if lastError.Valid { v := lastError.String; item.LastError = &v }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *app) queryTopVideos(ctx context.Context) ([]topVideoMetric, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT provider, provider_id, opens, related_clicks, source_clicks, favorites, hides FROM video_metrics ORDER BY (opens + related_clicks * 2 + source_clicks) DESC LIMIT 20`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []topVideoMetric{}
	for rows.Next() {
		var item topVideoMetric
		if err := rows.Scan(&item.Provider, &item.VideoID, &item.Opens, &item.RelatedClicks, &item.SourceClicks, &item.Favorites, &item.Hides); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *app) queryAds(ctx context.Context) ([]adMetric, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT day, slot, provider, impressions, clicks, revenue FROM monetization_metrics WHERE day >= DATE('now', '-30 day') ORDER BY day DESC, impressions DESC LIMIT 100`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []adMetric{}
	for rows.Next() {
		var item adMetric
		if err := rows.Scan(&item.Day, &item.Slot, &item.Provider, &item.Impressions, &item.Clicks, &item.Revenue); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func metricColumn(eventName string) string {
	switch eventName {
	case "video_impression": return "impressions"
	case "video_open": return "opens"
	case "related_click": return "related_clicks"
	case "source_click": return "source_clicks"
	case "favorite": return "favorites"
	case "hide": return "hides"
	default: return ""
	}
}

func sanitizeMetadata(input map[string]any) map[string]any {
	out := map[string]any{}
	count := 0
	for key, value := range input {
		if count >= 12 { break }
		key = truncate(key, 48)
		if key == "" { continue }
		switch v := value.(type) {
		case nil, bool, float64, float32, int, int64, json.Number:
			out[key] = v
		case string:
			out[key] = truncate(v, 160)
		default:
			out[key] = truncate(fmt.Sprint(v), 160)
		}
		count++
	}
	return out
}
