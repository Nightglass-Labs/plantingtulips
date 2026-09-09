package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type seoDocument struct {
	Title       string
	Description string
	Canonical   string
	Robots      string
	Image       string
	OGType      string
	Video       *VideoSummary
}

var seoRemovals = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<title>[\s\S]*?</title>`),
	regexp.MustCompile(`(?i)<meta\s+name=["']description["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+name=["']robots["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+property=["']og:title["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+property=["']og:description["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+property=["']og:type["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+property=["']og:url["'][^>]*>`),
	regexp.MustCompile(`(?i)<meta\s+property=["']og:image["'][^>]*>`),
	regexp.MustCompile(`(?i)<link\s+rel=["']canonical["'][^>]*>`),
	regexp.MustCompile(`(?is)<script\s+id=["']pt-server-video-jsonld["'][^>]*>.*?</script>`),
}

func (a *app) resolveSEO(ctx context.Context, requestURL *url.URL) seoDocument {
	pathname := requestURL.Path
	canonical := a.baseURL + pathname
	private := pathname == "/admin" || strings.HasPrefix(pathname, "/admin/") || pathname == "/library" || pathname == "/report"
	if private {
		title := "Utility · PlantingTulips"
		if strings.HasPrefix(pathname, "/admin") {
			title = "Admin · PlantingTulips"
		}
		if pathname == "/library" {
			title = "Your library · PlantingTulips"
		}
		return seoDocument{Title: title, Description: "PlantingTulips private or utility surface.", Canonical: canonical, Robots: "noindex,nofollow,noarchive", OGType: "website"}
	}

	if strings.HasPrefix(pathname, "/category/") {
		slug := cleanSegment(strings.TrimPrefix(pathname, "/category/"))
		name := titleCase(strings.ReplaceAll(slug, "-", " "))
		description := fmt.Sprintf("Browse %s videos in the PlantingTulips discovery catalog.", name)
		var dbName, dbDescription string
		if err := a.db.QueryRowContext(ctx, `SELECT name, description FROM categories WHERE slug = ? AND active = 1 LIMIT 1`, slug).Scan(&dbName, &dbDescription); err == nil {
			name, description = dbName, dbDescription
		}
		return seoDocument{Title: name + " videos · PlantingTulips", Description: description, Canonical: canonical, Robots: "index,follow,max-image-preview:large", OGType: "website"}
	}

	if strings.HasPrefix(pathname, "/tag/") {
		slug := cleanSegment(strings.TrimPrefix(pathname, "/tag/"))
		name := titleCase(strings.ReplaceAll(slug, "-", " "))
		return seoDocument{Title: name + " videos · PlantingTulips", Description: "Browse " + name + " videos across the PlantingTulips normalized catalog.", Canonical: canonical, Robots: "index,follow,max-image-preview:large", OGType: "website"}
	}

	if strings.HasPrefix(pathname, "/watch/") {
		parts := strings.Split(strings.Trim(pathname, "/"), "/")
		if len(parts) == 3 {
			provider, id := cleanSegment(parts[1]), cleanSegment(parts[2])
			if video, found, err := a.getCatalogVideo(ctx, provider, id); err == nil && found {
				return seoDocument{
					Title:       video.Title + " · PlantingTulips",
					Description: "Browse this video via the approved " + string(video.Provider) + " embed and discover related catalog results.",
					Canonical:   canonical, Robots: "index,follow,max-image-preview:large", Image: video.ThumbnailURL, OGType: "video.other", Video: &video,
				}
			}
		}
		return seoDocument{Title: "Video unavailable · PlantingTulips", Description: "This watch route has not been verified against the active PlantingTulips catalog.", Canonical: canonical, Robots: "noindex,follow", OGType: "video.other"}
	}

	if pathname == "/categories" {
		return seoDocument{Title: "Browse categories · PlantingTulips", Description: "Browse durable category hubs across the PlantingTulips discovery catalog.", Canonical: canonical, Robots: "index,follow,max-image-preview:large", OGType: "website"}
	}
	if legal := legalTitle(pathname); legal != "" {
		return seoDocument{Title: legal + " · PlantingTulips", Description: legal + " information for PlantingTulips.", Canonical: canonical, Robots: "index,follow", OGType: "website"}
	}
	if pathname != "/" {
		return seoDocument{Title: "Not found · PlantingTulips", Description: "This PlantingTulips page does not exist.", Canonical: canonical, Robots: "noindex,follow", OGType: "website"}
	}
	query := truncate(strings.ReplaceAll(strings.ReplaceAll(requestURL.Query().Get("q"), "<", ""), ">", ""), 120)
	if strings.EqualFold(query, "blowjob") {
		query = ""
	}
	if query != "" {
		canonical += "?q=" + url.QueryEscape(query)
		return seoDocument{Title: query + " videos · PlantingTulips", Description: "A focused video discovery catalog built from approved provider APIs, feeds, and embeds.", Canonical: canonical, Robots: "index,follow,max-image-preview:large", OGType: "website"}
	}
	return seoDocument{Title: "PlantingTulips · video discovery", Description: "A focused video discovery catalog built from approved provider APIs, feeds, and embeds.", Canonical: canonical, Robots: "index,follow,max-image-preview:large", OGType: "website"}
}

func injectSEO(input []byte, seo seoDocument) []byte {
	output := string(input)
	for _, re := range seoRemovals {
		output = re.ReplaceAllString(output, "")
	}
	tags := []string{
		"<title>" + html.EscapeString(seo.Title) + "</title>",
		`<meta name="description" content="` + html.EscapeString(seo.Description) + `" />`,
		`<meta name="robots" content="` + html.EscapeString(seo.Robots) + `" />`,
		`<link rel="canonical" href="` + html.EscapeString(seo.Canonical) + `" />`,
		`<meta property="og:title" content="` + html.EscapeString(seo.Title) + `" />`,
		`<meta property="og:description" content="` + html.EscapeString(seo.Description) + `" />`,
		`<meta property="og:type" content="` + html.EscapeString(defaultString(seo.OGType, "website")) + `" />`,
		`<meta property="og:url" content="` + html.EscapeString(seo.Canonical) + `" />`,
	}
	if seo.Image != "" {
		tags = append(tags, `<meta property="og:image" content="`+html.EscapeString(seo.Image)+`" />`)
	}
	if seo.Video != nil {
		body, _ := json.Marshal(map[string]any{
			"@context": "https://schema.org", "@type": "VideoObject",
			"name": seo.Video.Title, "description": seo.Description,
			"thumbnailUrl": []string{seo.Video.ThumbnailURL}, "embedUrl": seo.Video.EmbedURL,
		})
		tags = append(tags, `<script id="pt-server-video-jsonld" type="application/ld+json">`+string(body)+`</script>`)
	}
	output = strings.Replace(output, "</head>", "  "+strings.Join(tags, "\n  ")+"\n</head>", 1)
	return []byte(output)
}

func (a *app) robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /library\nDisallow: /report\nSitemap: %s/sitemap.xml\n", a.baseURL)
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}
type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (a *app) sitemap(w http.ResponseWriter, r *http.Request) {
	locations := []string{a.baseURL + "/", a.baseURL + "/categories"}
	categories, err := a.listCategories(r.Context())
	if err == nil {
		for _, category := range categories {
			locations = append(locations, a.baseURL+"/category/"+url.PathEscape(category.Slug))
		}
	}
	if rows, err := a.db.QueryContext(r.Context(), `
SELECT t.slug
FROM tags t
JOIN video_tags vt ON vt.tag_id = t.id
JOIN videos v ON v.id = vt.video_id
WHERE v.active = 1
GROUP BY t.id, t.slug
ORDER BY COUNT(*) DESC
LIMIT 250`); err == nil {
		for rows.Next() {
			var slug string
			if rows.Scan(&slug) == nil {
				locations = append(locations, a.baseURL+"/tag/"+url.PathEscape(slug))
			}
		}
		_ = rows.Close()
	}
	if rows, err := a.db.QueryContext(r.Context(), `
SELECT provider, provider_id FROM videos v
WHERE active = 1
AND NOT EXISTS (SELECT 1 FROM blocked_content b WHERE b.provider=v.provider AND b.provider_id=v.provider_id)
ORDER BY imported_at DESC LIMIT 1000`); err == nil {
		for rows.Next() {
			var provider, id string
			if rows.Scan(&provider, &id) == nil {
				locations = append(locations, a.baseURL+"/watch/"+url.PathEscape(provider)+"/"+url.PathEscape(id))
			}
		}
		_ = rows.Close()
	}
	set := sitemapSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: make([]sitemapURL, 0, len(locations))}
	for _, location := range locations {
		set.URLs = append(set.URLs, sitemapURL{Loc: location})
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		http.Error(w, "sitemap error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}

func cleanSegment(value string) string {
	decoded, err := url.PathUnescape(value)
	if err == nil {
		value = decoded
	}
	return truncate(value, 160)
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func legalTitle(path string) string {
	switch path {
	case "/terms":
		return "Terms"
	case "/privacy":
		return "Privacy"
	case "/dmca":
		return "DMCA"
	case "/2257":
		return "2257"
	default:
		return ""
	}
}

func nullableScanString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}
