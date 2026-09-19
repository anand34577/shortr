package server

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/public.html
var publicTemplatesFS embed.FS

var publicTemplates = template.Must(template.ParseFS(publicTemplatesFS, "templates/public.html"))

var pageTitles = map[string]string{
	"not_found": "Not found",
	"gone":      "Link expired",
	"disabled":  "Link disabled",
	"password":  "Password required",
}

// siteName is a static brand string for server-rendered public pages
// (not read from the settings table on every 404/410 — that would
// put a DB query on the redirect miss path; the SPA reads the live
// site_name setting via the API for the admin-configurable branding there).
const siteName = "Shortr"

func (s *Server) renderPublic(w http.ResponseWriter, status int, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	var body bytes.Buffer
	if err := publicTemplates.ExecuteTemplate(&body, name, data); err != nil {
		s.log.Error("template render failed", "template", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(status)
	_ = publicTemplates.ExecuteTemplate(w, "layout", map[string]any{
		"Title": pageTitles[name], "SiteName": siteName, "Body": template.HTML(body.String()), //nolint:gosec
	})
}
