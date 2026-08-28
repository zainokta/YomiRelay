package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:static
var assets embed.FS

// Handler serves the embedded frontend and falls back to its index for client routes.
func Handler() http.Handler {
	staticAssets, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(assets, "static/index.html")
	if err != nil {
		panic(err)
	}

	files := http.FileServer(http.FS(staticAssets))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if info, err := fs.Stat(staticAssets, name); err == nil && !info.IsDir() {
			files.ServeHTTP(response, request)
			return
		}
		if path.Ext(request.URL.Path) != "" {
			http.NotFound(response, request)
			return
		}

		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(index)
	})
}
