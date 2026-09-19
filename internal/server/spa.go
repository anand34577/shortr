package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the embedded React build with an SPA history fallback:
// any /app/* path that doesn't match a real file returns index.html so
// client-side routing works on a hard refresh / deep link.
func (s *Server) spaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.spaFS == nil {
			http.Error(w, "frontend not built into this binary", http.StatusNotImplemented)
			return
		}
		upath := strings.TrimPrefix(r.URL.Path, "/app")
		upath = strings.TrimPrefix(upath, "/")
		if upath == "" {
			upath = "index.html"
		}
		clean := path.Clean(upath)
		if strings.HasPrefix(clean, "..") {
			http.NotFound(w, r)
			return
		}

		f, err := s.spaFS.Open(clean)
		if err != nil {
			// not a real asset: SPA route -> serve index.html
			serveIndex(w, r, s.spaFS)
			return
		}
		f.Close()

		if isHashedAsset(clean) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, r, s.spaFS, clean)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	w.Header().Set("Cache-Control", "no-cache")
	f, err := fsys.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f.Close()
	http.ServeFileFS(w, r, fsys, "index.html")
}

// isHashedAsset guesses whether a build artifact has a content hash in its
// filename (Vite's default output: name.<hash>.ext) and can be cached
// forever; index.html and unhashed files must always revalidate.
func isHashedAsset(p string) bool {
	base := path.Base(p)
	if base == "index.html" {
		return false
	}
	return strings.Contains(base, "-") || strings.Contains(base, ".") && strings.Count(base, ".") >= 2
}
