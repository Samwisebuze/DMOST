package http

import (
	"embed"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed web/index.html web/app.css web/app.js web/sw.js web/icon.svg web/manifest.webmanifest
var webAssets embed.FS

func (s *Server) registerLocalWeb() {
	s.router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		switch name {
		case "index.html", "app.css", "app.js", "sw.js", "icon.svg", "manifest.webmanifest":
		default:
			s.handleNotFound(w, r)
			return
		}
		data, err := webAssets.ReadFile("web/" + name)
		if err != nil {
			s.handleNotFound(w, r)
			return
		}
		contentType := mime.TypeByExtension(path.Ext(name))
		if name == "manifest.webmanifest" {
			contentType = "application/manifest+json"
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if name == "sw.js" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		_, _ = w.Write(data)
	})).Methods(http.MethodGet)
}
