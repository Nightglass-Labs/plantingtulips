package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ProviderName string

const (
	ProviderEporner ProviderName = "eporner"
	ProviderXvideos ProviderName = "xvideos"
)

type VideoSummary struct {
	ID           string       `json:"id"`
	Provider     ProviderName `json:"provider"`
	Title        string       `json:"title"`
	ThumbnailURL string       `json:"thumbnailUrl"`
	Duration     string       `json:"duration"`
	Views        *int64       `json:"views,omitempty"`
	Rating       *float64     `json:"rating,omitempty"`
	SourceURL    string       `json:"sourceUrl"`
	EmbedURL     string       `json:"embedUrl"`
	Tags         []string     `json:"tags,omitempty"`
}

type providerSearchResult struct {
	Page       int
	TotalPages int
	TotalCount int
	Videos     []VideoSummary
}

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (a *app) searchEporner(ctx context.Context, query string, page int, order string) (providerSearchResult, error) {
	u, _ := url.Parse("https://www.eporner.com/api/v2/video/search/")
	q := u.Query()
	q.Set("query", query)
	q.Set("per_page", "48")
	q.Set("page", strconv.Itoa(page))
	q.Set("thumbsize", "big")
	q.Set("order", normalizeProviderOrder(order))
	q.Set("gay", "1")
	q.Set("lq", "0")
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	body, err := a.fetchProvider(ctx, u, "Eporner API", nil)
	if err != nil {
		return providerSearchResult{}, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return providerSearchResult{}, fmt.Errorf("decode Eporner search: %w", err)
	}
	result := providerSearchResult{
		Page:       int(numberValue(data["page"], float64(page))),
		TotalPages: max(1, int(numberValue(data["total_pages"], 1))),
		TotalCount: int(numberValue(data["total_count"], 0)),
		Videos:     []VideoSummary{},
	}
	if values, ok := data["videos"].([]any); ok {
		for _, value := range values {
			if raw, ok := value.(map[string]any); ok {
				video := normalizeEporner(raw)
				if validVideo(video) {
					result.Videos = append(result.Videos, video)
				}
			}
		}
	}
	return result, nil
}

func (a *app) getEporner(ctx context.Context, id string) (VideoSummary, error) {
	u, _ := url.Parse("https://www.eporner.com/api/v2/video/id/")
	q := u.Query()
	q.Set("id", id)
	q.Set("thumbsize", "big")
	q.Set("format", "json")
	u.RawQuery = q.Encode()
	body, err := a.fetchProvider(ctx, u, "Eporner API", nil)
	if err != nil {
		return VideoSummary{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return VideoSummary{}, fmt.Errorf("decode Eporner video: %w", err)
	}
	video := normalizeEporner(raw)
	if !validVideo(video) {
		return VideoSummary{}, fmt.Errorf("Eporner returned an invalid video")
	}
	return video, nil
}

func (a *app) getEpornerRemoved(ctx context.Context) ([]string, error) {
	u, _ := url.Parse("https://www.eporner.com/api/v2/video/removed/")
	q := u.Query()
	q.Set("format", "json")
	u.RawQuery = q.Encode()
	body, err := a.fetchProvider(ctx, u, "Eporner removed API", nil)
	if err != nil {
		return nil, err
	}

	ids := []string{}
	var decoded any
	if json.Unmarshal(body, &decoded) == nil {
		var values []any
		switch value := decoded.(type) {
		case []any:
			values = value
		case map[string]any:
			for _, key := range []string{"removed", "videos", "ids", "list"} {
				if list, ok := value[key].([]any); ok {
					values = list
					break
				}
			}
		}
		for _, value := range values {
			id := ""
			switch item := value.(type) {
			case string:
				id = item
			case map[string]any:
				id = stringValue(item["id"])
			}
			id = strings.TrimSpace(id)
			if providerIDPattern.MatchString(id) {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	for _, value := range regexp.MustCompile(`[\r\n,]+`).Split(string(body), -1) {
		value = strings.TrimSpace(value)
		if providerIDPattern.MatchString(value) {
			ids = append(ids, value)
		}
	}
	return ids, nil
}

func (a *app) searchXvideos(ctx context.Context, query string, page int) (providerSearchResult, error) {
	if a.xvideosEndpoint == "" {
		return providerSearchResult{Page: page, TotalPages: 1, Videos: []VideoSummary{}}, nil
	}
	u, err := url.Parse(a.xvideosEndpoint)
	if err != nil || u.Scheme != "https" {
		return providerSearchResult{}, fmt.Errorf("XVideos partner endpoint must be HTTPS")
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	headers := map[string]string{}
	if a.xvideosAuthorization != "" {
		headers["Authorization"] = a.xvideosAuthorization
	}
	body, err := a.fetchProvider(ctx, u, "XVideos partner endpoint", headers)
	if err != nil {
		return providerSearchResult{}, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return providerSearchResult{}, fmt.Errorf("decode XVideos partner response: %w", err)
	}
	result := providerSearchResult{Page: page, TotalPages: 1, Videos: []VideoSummary{}}
	if values, ok := data["videos"].([]any); ok {
		for _, value := range values {
			if raw, ok := value.(map[string]any); ok {
				video := normalizeXvideos(raw)
				if validVideo(video) {
					result.Videos = append(result.Videos, video)
				}
			}
		}
	}
	return result, nil
}

func (a *app) getXvideos(ctx context.Context, id string) (VideoSummary, error) {
	if a.xvideosEndpoint == "" {
		return VideoSummary{}, fmt.Errorf("XVideos partner feed is not configured")
	}
	u, err := url.Parse(a.xvideosEndpoint)
	if err != nil || u.Scheme != "https" {
		return VideoSummary{}, fmt.Errorf("XVideos partner endpoint must be HTTPS")
	}
	q := u.Query()
	q.Set("id", id)
	u.RawQuery = q.Encode()
	headers := map[string]string{}
	if a.xvideosAuthorization != "" {
		headers["Authorization"] = a.xvideosAuthorization
	}
	body, err := a.fetchProvider(ctx, u, "XVideos partner endpoint", headers)
	if err != nil {
		return VideoSummary{}, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return VideoSummary{}, fmt.Errorf("decode XVideos partner response: %w", err)
	}
	var raw map[string]any
	if video, ok := data["video"].(map[string]any); ok {
		raw = video
	} else if videos, ok := data["videos"].([]any); ok && len(videos) > 0 {
		raw, _ = videos[0].(map[string]any)
	}
	if raw == nil {
		return VideoSummary{}, fmt.Errorf("XVideos video not found in partner feed")
	}
	video := normalizeXvideos(raw)
	if !validVideo(video) {
		return VideoSummary{}, fmt.Errorf("XVideos partner payload did not include valid HTTPS source/embed URLs")
	}
	return video, nil
}

func (a *app) fetchProvider(ctx context.Context, target *url.URL, label string, headers map[string]string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json, text/plain;q=0.8")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := a.client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return body, nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("%s returned %d", label, resp.StatusCode)
				if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
					return nil, lastErr
				}
			}
		} else {
			lastErr = err
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s unavailable", label)
	}
	return nil, lastErr
}

func normalizeEporner(raw map[string]any) VideoSummary {
	id := stringValue(raw["id"])
	seconds := int(numberValue(raw["length_sec"], 0))
	thumb := stringValue(raw["thumb"])
	if value, ok := raw["default_thumb"].(map[string]any); ok {
		thumb = stringValue(value["src"])
	} else if value := stringValue(raw["default_thumb"]); value != "" {
		thumb = value
	}
	duration := stringValue(raw["length_min"])
	if duration == "" {
		duration = formatDuration(seconds)
	}
	tags := []string{}
	for _, tag := range strings.Split(stringValue(raw["keywords"]), ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" && len(tags) < 20 {
			tags = append(tags, tag)
		}
	}
	views := optionalInt64(raw["views"])
	rating := optionalPercent(raw["rate"])
	source := stringValue(raw["url"])
	if source == "" && id != "" {
		source = "https://www.eporner.com/video-" + id + "/"
	}
	embed := stringValue(raw["embed"])
	if embed == "" && id != "" {
		embed = "https://www.eporner.com/embed/" + id + "/"
	}
	return VideoSummary{ID: id, Provider: ProviderEporner, Title: defaultString(stringValue(raw["title"]), "Untitled"), ThumbnailURL: thumb, Duration: duration, Views: views, Rating: rating, SourceURL: source, EmbedURL: embed, Tags: tags}
}

func normalizeXvideos(raw map[string]any) VideoSummary {
	tags := []string{}
	if values, ok := raw["tags"].([]any); ok {
		for _, value := range values {
			tag := strings.TrimSpace(stringValue(value))
			if tag != "" && len(tags) < 20 {
				tags = append(tags, tag)
			}
		}
	}
	return VideoSummary{
		ID:           stringValue(raw["id"]),
		Provider:     ProviderXvideos,
		Title:        defaultString(stringValue(raw["title"]), "Untitled"),
		ThumbnailURL: firstString(raw, "thumbnailUrl", "thumbnail"),
		Duration:     stringValue(raw["duration"]),
		Views:        optionalInt64(raw["views"]),
		Rating:       optionalPercent(raw["rating"]),
		SourceURL:    firstString(raw, "sourceUrl", "url"),
		EmbedURL:     firstString(raw, "embedUrl", "embed"),
		Tags:         tags,
	}
}

func validVideo(video VideoSummary) bool {
	if video.ID == "" || strings.TrimSpace(video.Title) == "" {
		return false
	}
	return validHTTPS(video.SourceURL) && validHTTPS(video.EmbedURL)
}

func validHTTPS(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func normalizeProviderOrder(value string) string {
	switch value {
	case "latest", "longest", "shortest", "top-rated", "most-popular", "top-weekly", "top-monthly":
		return value
	default:
		return "top-weekly"
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func numberValue(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if n, err := v.Float64(); err == nil {
			return n
		}
	case string:
		clean := regexp.MustCompile(`[^0-9.\-]+`).ReplaceAllString(v, "")
		if n, err := strconv.ParseFloat(clean, 64); err == nil {
			return n
		}
	}
	return fallback
}

func optionalInt64(value any) *int64 {
	n := int64(numberValue(value, 0))
	if n <= 0 {
		return nil
	}
	return &n
}

func optionalPercent(value any) *float64 {
	n := numberValue(value, 0)
	if n <= 0 {
		return nil
	}
	if n <= 5 {
		n *= 20
	}
	if n > 100 {
		n = 100
	}
	return &n
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(raw[key]); value != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	minutes := seconds / 60
	return fmt.Sprintf("%d:%02d", minutes, seconds%60)
}
