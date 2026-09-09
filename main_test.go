package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "turso.tech/database/tursogo"
)

func testApp(t *testing.T) *app {
	t.Helper()
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	a, err := newApp(db)
	if err != nil {
		t.Fatal(err)
	}
	a.adminToken = "admin-test"
	a.ingestSecret = "ingest-test"
	a.baseURL = "https://example.com"
	return a
}

func TestHealthChecksTurso(t *testing.T) {
	a := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	res := httptest.NewRecorder()
	a.health(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"ok":true`) {
		t.Fatalf("health status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestMigrationsSeedProviderAndCategories(t *testing.T) {
	a := testApp(t)
	var migrations, categories, providers int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", migrations)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM categories WHERE active=1`).Scan(&categories); err != nil {
		t.Fatal(err)
	}
	if categories < 6 {
		t.Fatalf("expected seeded categories, got %d", categories)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM providers`).Scan(&providers); err != nil {
		t.Fatal(err)
	}
	if providers != 2 {
		t.Fatalf("expected 2 providers, got %d", providers)
	}
}

func TestReportCreatesUrgentSafetyHold(t *testing.T) {
	a := testApp(t)
	body := strings.NewReader(`{"email":"reporter@example.com","reason":"minor","provider":"eporner","videoId":"abc123","sourceUrl":"https://example.com/video","details":"urgent safety report"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/report", body)
	res := httptest.NewRecorder()
	a.handleReport(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("report status=%d body=%s", res.Code, res.Body.String())
	}
	var response struct {
		Stored      bool `json:"stored"`
		AutoBlocked bool `json:"autoBlocked"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Stored || !response.AutoBlocked {
		t.Fatalf("unexpected report response: %#v", response)
	}
	blocked, err := a.isBlocked(req.Context(), "eporner", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("urgent report did not create local safety hold")
	}
}

func TestAnalyticsFeedsDashboardAndRetention(t *testing.T) {
	a := testApp(t)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	for _, input := range []AnalyticsInput{
		{SessionID: "session_test_01", EventName: "video_impression", Provider: "eporner", VideoID: "one", Metadata: map[string]any{"context": "feed"}},
		{SessionID: "session_test_01", EventName: "video_open", Provider: "eporner", VideoID: "one"},
		{SessionID: "session_test_01", EventName: "video_impression", Provider: "eporner", VideoID: "two", Metadata: map[string]any{"context": "related"}},
		{SessionID: "session_test_01", EventName: "related_click", Provider: "eporner", VideoID: "two"},
	} {
		if err := a.recordAnalyticsEvent(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	dashboard, err := a.adminDashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Funnel7d.Sessions != 1 || dashboard.Funnel7d.FeedImpressions != 1 || dashboard.Funnel7d.VideoOpens != 1 || dashboard.Funnel7d.RelatedImpressions != 1 || dashboard.Funnel7d.RelatedClicks != 1 {
		t.Fatalf("unexpected funnel: %#v", dashboard.Funnel7d)
	}
	result, err := a.pruneAnalytics(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	if result["remainingEvents"].(int64) != 4 {
		t.Fatalf("unexpected retention result: %#v", result)
	}
}

func TestProviderKillSwitch(t *testing.T) {
	a := testApp(t)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	enabled, err := a.providerEnabled(ctx, ProviderEporner)
	if err != nil || !enabled {
		t.Fatalf("expected eporner enabled, enabled=%v err=%v", enabled, err)
	}
	if err := a.setProviderEnabled(ctx, ProviderEporner, false); err != nil {
		t.Fatal(err)
	}
	enabled, err = a.providerEnabled(ctx, ProviderEporner)
	if err != nil || enabled {
		t.Fatalf("expected eporner disabled, enabled=%v err=%v", enabled, err)
	}
}

func TestSPAFallbackAndPrivateNoindex(t *testing.T) {
	a := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	res := httptest.NewRecorder()
	a.spa(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("spa status=%d body=%s", res.Code, res.Body.String())
	}
	if res.Header().Get("X-PT-SEO") != "go" {
		t.Fatalf("expected Go SEO header, got %q", res.Header().Get("X-PT-SEO"))
	}
	if !strings.Contains(res.Body.String(), `noindex,nofollow,noarchive`) {
		t.Fatalf("private route was not noindex: %s", res.Body.String())
	}
}

func TestAdminRequiresBearerToken(t *testing.T) {
	a := testApp(t)
	handler := a.requireAdmin(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	unauthorized := httptest.NewRecorder()
	handler(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorized.Code)
	}
	authorizedReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	authorizedReq.Header.Set("Authorization", "Bearer admin-test")
	authorized := httptest.NewRecorder()
	handler(authorized, authorizedReq)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("expected authorized request, got %d", authorized.Code)
	}
}
