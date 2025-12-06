package service

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NotFoundHandler wraps a file server to serve a custom 404 page for non-existent routes
func NotFoundHandler(root http.FileSystem) http.Handler {
	fs := http.FileServer(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := filepath.Clean(r.URL.Path)

		// Try to open the file
		fullPath := filepath.Join("./frontend/out", path)

		// Check if it's a directory, if so try index.html
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			indexPath := filepath.Join(fullPath, "index.html")
			if _, err := os.Stat(indexPath); err != nil {
				// No index.html in directory, serve 404
				serve404(w, r)
				return
			}
		} else if err != nil {
			// File doesn't exist
			// Check if it's not an API or static asset request
			if !strings.HasPrefix(path, "/api/") &&
				!strings.HasPrefix(path, "/_next/") &&
				!strings.HasPrefix(path, "/static/") &&
				!strings.Contains(path, ".") { // Likely a route, not a file
				serve404(w, r)
				return
			}
		}

		// File exists or is a static asset, serve it
		fs.ServeHTTP(w, r)
	})
}

// serve404 serves the custom 404.html page
func serve404(w http.ResponseWriter, r *http.Request) {
	notFoundPath := "./frontend/out/404.html"
	content, err := os.ReadFile(notFoundPath)
	if err != nil {
		// Fallback if 404.html doesn't exist
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 - Page Not Found"))
		return
	}

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}
