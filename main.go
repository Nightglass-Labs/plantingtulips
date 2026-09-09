package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

//go:embed dist
var webFS embed.FS

type app struct {
	db                  *sql.DB
	adminToken          string
	ingestSecret         string
	baseURL              string
	xvideosEndpoint      string
	xvideosAuthorization string
	reportWebhookURL     string
	client               *http.Client
	dist                 fs.FS
	static               http.Handler
	indexHTML            []byte
}

func main() {
	ctx := context.Background()
	db, err := openDatabase(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	a, err := newApp(db)
	if err != nil {
		log.Fatal(err)
	}

	addr := env("ADDR", ":8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           logRequests(securityHeaders(a.routes())),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("plantingtulips listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func newApp(db *sql.DB) (*app, error) {
	dist, err := fs.Sub(webFS, "dist")
	if err != nil {
		return nil, err
	}
	indexHTML, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, err
	}
	adminToken := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	ingestSecret := strings.TrimSpace(os.Getenv("INGEST_SECRET"))
	if adminToken == "" {
		adminToken = ingestSecret
	}
	return &app{
		db:                  db,
		adminToken:          adminToken,
		ingestSecret:         ingestSecret,
		baseURL:              strings.TrimRight(env("BASE_URL", "https://plantingtulips.com"), "/"),
		xvideosEndpoint:      strings.TrimSpace(os.Getenv("XVIDEOS_PARTNER_JSON_ENDPOINT")),
		xvideosAuthorization: strings.TrimSpace(os.Getenv("XVIDEOS_PARTNER_AUTHORIZATION")),
		reportWebhookURL:     strings.TrimSpace(os.Getenv("REPORT_WEBHOOK_URL")),
		client:               &http.Client{Timeout: 10 * time.Second},
		dist:                 dist,
		static:               http.FileServer(http.FS(dist)),
		indexHTML:            indexHTML,
	}, nil
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/healthz", a.health)
	mux.HandleFunc("GET /api/v1/videos", a.handleVideos)
	mux.HandleFunc("GET /api/v1/video/{provider}/{id}", a.handleVideo)
	mux.HandleFunc("GET /api/v1/related/{provider}/{id}", a.handleRelated)
	mux.HandleFunc("GET /api/v1/categories", a.handleCategories)
	mux.HandleFunc("POST /api/v1/events", a.handleEvent)
	mux.HandleFunc("POST /api/v1/report", a.handleReport)

	mux.HandleFunc("GET /api/v1/admin/dashboard", a.requireAdmin(a.handleAdminDashboard))
	mux.HandleFunc("GET /api/v1/admin/reports", a.requireAdmin(a.handleAdminReports))
	mux.HandleFunc("POST /api/v1/admin/block", a.requireAdmin(a.handleAdminBlock))
	mux.HandleFunc("POST /api/v1/admin/provider", a.requireAdmin(a.handleAdminProvider))
	mux.HandleFunc("POST /api/v1/admin/maintenance", a.requireAdmin(a.handleAdminMaintenance))
	mux.HandleFunc("POST /api/v1/admin/ingest", a.requireIngest(a.handleAdminIngest))

	mux.HandleFunc("GET /robots.txt", a.robots)
	mux.HandleFunc("GET /sitemap.xml", a.sitemap)
	mux.HandleFunc("/", a.spa)
	return mux
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "database": "unavailable"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "database": "ok"})
}

func (a *app) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.adminToken == "" {
			jsonError(w, http.StatusServiceUnavailable, "ADMIN_TOKEN is not configured")
			return
		}
		if bearer(r) != a.adminToken {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (a *app) requireIngest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if a.ingestSecret == "" && a.adminToken == "" {
			jsonError(w, http.StatusServiceUnavailable, "INGEST_SECRET is not configured")
			return
		}
		if token == "" || (token != a.ingestSecret && token != a.adminToken) {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (a *app) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean != "." && clean != "index.html" {
		if f, err := a.dist.Open(clean); err == nil {
			_ = f.Close()
			a.static.ServeHTTP(w, r)
			return
		}
	}

	seo := a.resolveSEO(r.Context(), r.URL)
	pageHTML := injectSEO(a.indexHTML, seo)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-PT-SEO", "go")
	_, _ = w.Write(pageHTML)
}

func bearer(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return strings.TrimSpace(r.Header.Get("X-Admin-Token"))
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; connect-src 'self'; frame-src https://www.eporner.com https://www.xvideos.com; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
