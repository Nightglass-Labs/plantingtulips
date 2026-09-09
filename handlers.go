package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type videoPageResponse struct {
	Query      string                          `json:"query"`
	Page       int                             `json:"page"`
	TotalPages int                             `json:"totalPages"`
	TotalCount int                             `json:"totalCount"`
	Videos     []VideoSummary                  `json:"videos"`
	Source     string                          `json:"source,omitempty"`
	Providers  map[ProviderName]ProviderStatus `json:"providers"`
}

func (a *app) handleVideos(w http.ResponseWriter, r *http.Request) {
	query := truncate(strings.TrimSpace(r.URL.Query().Get("q")), 160)
	if query == "" {
		query = "blowjob"
	}
	page := parseBoundedInt(r.URL.Query().Get("page"), 1, 1, 10000)
	sort := normalizeProviderOrder(r.URL.Query().Get("sort"))

	statuses, err := a.providerStatuses(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "database error")
		return
	}
	catalog, err := a.searchCatalog(r.Context(), query, page, sort)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "catalog error")
		return
	}
	if len(catalog.Videos) > 0 || catalog.TotalCount > 0 {
		jsonResponse(w, http.StatusOK, videoPageResponse{Query: query, Page: catalog.Page, TotalPages: catalog.TotalPages, TotalCount: catalog.TotalCount, Videos: catalog.Videos, Source: "catalog", Providers: statuses})
		return
	}

	live := providerSearchResult{Page: page, TotalPages: 1, Videos: []VideoSummary{}}
	for _, provider := range []ProviderName{ProviderEporner, ProviderXvideos} {
		status := statuses[provider]
		if !status.Enabled {
			continue
		}
		var result providerSearchResult
		var providerErr error
		switch provider {
		case ProviderEporner:
			result, providerErr = a.searchEporner(r.Context(), query, page, sort)
		case ProviderXvideos:
			result, providerErr = a.searchXvideos(r.Context(), query, page)
		}
		if providerErr != nil {
			_ = a.recordProviderHealth(r.Context(), provider, false, providerErr.Error())
			status.Note = providerErr.Error()
			statuses[provider] = status
			continue
		}
		_ = a.recordProviderHealth(r.Context(), provider, true, "")
		live.TotalCount += result.TotalCount
		live.TotalPages = max(live.TotalPages, result.TotalPages)
		for _, video := range result.Videos {
			blocked, _ := a.isBlocked(r.Context(), string(video.Provider), video.ID)
			if !blocked && len(live.Videos) < 48 {
				live.Videos = append(live.Videos, video)
			}
		}
	}
	jsonResponse(w, http.StatusOK, videoPageResponse{Query: query, Page: page, TotalPages: live.TotalPages, TotalCount: live.TotalCount, Videos: live.Videos, Source: "live", Providers: statuses})
}

func (a *app) handleVideo(w http.ResponseWriter, r *http.Request) {
	provider := ProviderName(strings.ToLower(truncate(r.PathValue("provider"), 32)))
	id := truncate(r.PathValue("id"), 160)
	if !validProvider(provider) || id == "" {
		jsonError(w, http.StatusBadRequest, "invalid provider or id")
		return
	}
	blocked, err := a.isBlocked(r.Context(), string(provider), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "database error")
		return
	}
	if blocked {
		jsonError(w, http.StatusNotFound, "video not found")
		return
	}
	if video, found, err := a.getCatalogVideo(r.Context(), string(provider), id); err != nil {
		jsonError(w, http.StatusInternalServerError, "database error")
		return
	} else if found {
		jsonResponse(w, http.StatusOK, video)
		return
	}
	enabled, err := a.providerEnabled(r.Context(), provider)
	if err != nil || !enabled {
		jsonError(w, http.StatusNotFound, "video not found")
		return
	}
	var video VideoSummary
	switch provider {
	case ProviderEporner:
		video, err = a.getEporner(r.Context(), id)
	case ProviderXvideos:
		video, err = a.getXvideos(r.Context(), id)
	}
	if err != nil {
		_ = a.recordProviderHealth(r.Context(), provider, false, err.Error())
		jsonError(w, http.StatusBadGateway, "provider unavailable")
		return
	}
	_ = a.recordProviderHealth(r.Context(), provider, true, "")
	_ = a.upsertVideo(r.Context(), video, "")
	jsonResponse(w, http.StatusOK, video)
}

func (a *app) handleRelated(w http.ResponseWriter, r *http.Request) {
	provider := truncate(r.PathValue("provider"), 32)
	id := truncate(r.PathValue("id"), 160)
	videos, err := a.relatedVideos(r.Context(), provider, id, 12)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "related-video query failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"videos": videos})
}

func (a *app) handleCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listCategories(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "category query failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"categories": categories, "configured": true})
}

func (a *app) handleEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()
	var input AnalyticsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid event")
		return
	}
	limited, err := a.analyticsRateLimited(r.Context(), input.SessionID, 90)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "analytics unavailable")
		return
	}
	if limited {
		jsonError(w, http.StatusTooManyRequests, "analytics rate limit")
		return
	}
	if err := a.recordAnalyticsEvent(r.Context(), input); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) handleReport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 24<<10)
	defer r.Body.Close()
	var input struct {
		Email     string `json:"email"`
		Reason    string `json:"reason"`
		Provider  string `json:"provider"`
		VideoID   string `json:"videoId"`
		SourceURL string `json:"sourceUrl"`
		Details   string `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid report")
		return
	}
	input.Email = truncate(input.Email, 320)
	input.Reason = truncate(input.Reason, 32)
	input.Provider = truncate(input.Provider, 40)
	input.VideoID = truncate(input.VideoID, 120)
	input.SourceURL = truncate(input.SourceURL, 2000)
	input.Details = truncate(input.Details, 10000)
	validReasons := map[string]bool{"ncii": true, "minor": true, "deepfake": true, "copyright": true, "other": true}
	if !validEmail(input.Email) || !validReasons[input.Reason] || input.Details == "" {
		jsonError(w, http.StatusBadRequest, "invalid report")
		return
	}
	reportID, err := randomID()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "report id error")
		return
	}
	report := Report{ID: reportID, Email: input.Email, Reason: input.Reason, Provider: input.Provider, VideoID: input.VideoID, SourceURL: input.SourceURL, Details: input.Details}
	if err := a.saveReport(r.Context(), report); err != nil {
		jsonError(w, http.StatusInternalServerError, "report storage failed")
		return
	}
	autoBlocked := false
	if (input.Reason == "ncii" || input.Reason == "minor") && input.Provider != "" && input.VideoID != "" {
		if err := a.blockContent(r.Context(), input.Provider, input.VideoID, "automatic safety hold: "+input.Reason, reportID); err == nil {
			autoBlocked = true
		}
	}
	relayed := false
	if a.reportWebhookURL != "" {
		relayed = a.relayReport(r.Context(), report, autoBlocked)
	}
	jsonResponse(w, http.StatusAccepted, map[string]any{"ok": true, "reportId": reportID, "stored": true, "relayed": relayed, "autoBlocked": autoBlocked})
}

func (a *app) relayReport(ctx context.Context, report Report, autoBlocked bool) bool {
	if _, err := url.ParseRequestURI(a.reportWebhookURL); err != nil {
		return false
	}
	payload, _ := json.Marshal(map[string]any{
		"reportId": report.ID, "email": report.Email, "reason": report.Reason,
		"provider": report.Provider, "videoId": report.VideoID, "sourceUrl": report.SourceURL,
		"details": report.Details, "receivedAt": time.Now().UTC().Format(time.RFC3339), "autoBlocked": autoBlocked,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.reportWebhookURL, bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (a *app) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := a.adminDashboard(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "dashboard query failed")
		return
	}
	jsonResponse(w, http.StatusOK, data)
}

func (a *app) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	reports, err := a.listReports(r.Context(), parseBoundedInt(r.URL.Query().Get("limit"), 100, 1, 250))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "report query failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"reports": reports})
}

func (a *app) handleAdminBlock(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider string `json:"provider"`
		VideoID  string `json:"videoId"`
		Reason   string `json:"reason"`
		ReportID string `json:"reportId"`
	}
	if decodeJSONBody(w, r, 12<<10, &input) != nil {
		jsonError(w, http.StatusBadRequest, "invalid block")
		return
	}
	input.Provider = truncate(input.Provider, 40)
	input.VideoID = truncate(input.VideoID, 160)
	input.Reason = truncate(input.Reason, 1000)
	input.ReportID = truncate(input.ReportID, 128)
	if input.Provider == "" || input.VideoID == "" || input.Reason == "" {
		jsonError(w, http.StatusBadRequest, "provider, videoId, and reason are required")
		return
	}
	if err := a.blockContent(r.Context(), input.Provider, input.VideoID, input.Reason, input.ReportID); err != nil {
		jsonError(w, http.StatusInternalServerError, "block failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) handleAdminProvider(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider ProviderName `json:"provider"`
		Enabled  bool         `json:"enabled"`
	}
	if decodeJSONBody(w, r, 4<<10, &input) != nil || !validProvider(input.Provider) {
		jsonError(w, http.StatusBadRequest, "invalid provider")
		return
	}
	if input.Provider == ProviderXvideos && input.Enabled && a.xvideosEndpoint == "" {
		jsonError(w, http.StatusConflict, "XVideos approved partner endpoint is not configured")
		return
	}
	if err := a.setProviderEnabled(r.Context(), input.Provider, input.Enabled); err != nil {
		jsonError(w, http.StatusInternalServerError, "provider update failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) handleAdminMaintenance(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RetentionDays int `json:"retentionDays"`
	}
	_ = decodeJSONBody(w, r, 4<<10, &input)
	if input.RetentionDays == 0 {
		input.RetentionDays = 90
	}
	analytics, err := a.pruneAnalytics(r.Context(), input.RetentionDays)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "maintenance failed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "analytics": analytics})
}

func (a *app) handleAdminIngest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Pages       int  `json:"pages"`
		SyncRemoved bool `json:"syncRemoved"`
	}
	_ = decodeJSONBody(w, r, 4<<10, &input)
	if input.Pages == 0 {
		input.Pages = 1
	}
	input.Pages = max(1, min(5, input.Pages))
	result, err := a.runIngestion(r.Context(), input.Pages, input.SyncRemoved)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

type ingestProviderResult struct {
	Imported int      `json:"imported"`
	Removed  int      `json:"removed"`
	Errors   []string `json:"errors"`
}

func (a *app) runIngestion(ctx context.Context, pages int, syncRemoved bool) (map[string]any, error) {
	categories, err := a.listCategories(ctx)
	if err != nil {
		return nil, err
	}
	results := map[ProviderName]*ingestProviderResult{
		ProviderEporner: {Errors: []string{}},
		ProviderXvideos: {Errors: []string{}},
	}
	for _, provider := range []ProviderName{ProviderEporner, ProviderXvideos} {
		enabled, err := a.providerEnabled(ctx, provider)
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		for _, category := range categories {
			for page := 1; page <= pages; page++ {
				var search providerSearchResult
				var searchErr error
				switch provider {
				case ProviderEporner:
					search, searchErr = a.searchEporner(ctx, category.Query, page, "top-weekly")
				case ProviderXvideos:
					search, searchErr = a.searchXvideos(ctx, category.Query, page)
				}
				if searchErr != nil {
					results[provider].Errors = append(results[provider].Errors, category.Slug+": "+searchErr.Error())
					_ = a.recordProviderHealth(ctx, provider, false, searchErr.Error())
					break
				}
				_ = a.recordProviderHealth(ctx, provider, true, "")
				for _, video := range search.Videos {
					if err := a.upsertVideo(ctx, video, category.Slug); err != nil {
						results[provider].Errors = append(results[provider].Errors, "upsert: "+err.Error())
						continue
					}
					results[provider].Imported++
				}
			}
		}
		removedSynced := false
		if provider == ProviderEporner && syncRemoved {
			ids, removedErr := a.getEpornerRemoved(ctx)
			if removedErr != nil {
				results[provider].Errors = append(results[provider].Errors, "removed sync: "+removedErr.Error())
			} else {
				if err := a.deactivateProviderIDs(ctx, provider, ids); err != nil {
					results[provider].Errors = append(results[provider].Errors, "removed apply: "+err.Error())
				} else {
					results[provider].Removed = len(ids)
					removedSynced = true
				}
			}
		}
		message := ""
		if len(results[provider].Errors) > 0 {
			message = strings.Join(results[provider].Errors, "; ")
		}
		_ = a.markSync(ctx, provider, results[provider].Imported, removedSynced, message)
	}
	return map[string]any{"ok": true, "pages": pages, "providers": results}, nil
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func parseBoundedInt(value string, fallback, low, high int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return max(low, min(high, n))
}

func validProvider(provider ProviderName) bool {
	return provider == ProviderEporner || provider == ProviderXvideos
}

func validEmail(value string) bool {
	at := strings.LastIndex(value, "@")
	return at > 0 && at < len(value)-3 && strings.Contains(value[at+1:], ".")
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func contextError(ctx context.Context, fallback string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%s", fallback)
}
