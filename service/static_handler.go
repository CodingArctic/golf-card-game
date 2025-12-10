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

	// Get the absolute base path once during initialization
	basePath, err := filepath.Abs("./frontend/out")
	if err != nil {
		// If we can't get the base path, use a safer default
		basePath = "./frontend/out"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path to prevent directory traversal
		path := filepath.Clean(r.URL.Path)

		// Remove leading slash and join with base path
		relativePath := strings.TrimPrefix(path, "/")
		fullPath := filepath.Join(basePath, relativePath)

		// Security check: ensure the resolved path is within the base directory
		absPath, err := filepath.Abs(fullPath)
		if err != nil || !strings.HasPrefix(absPath, basePath) {
			// Path traversal attempt detected
			serve404(w)
			return
		}

		// Check if it's a directory, if so try index.html
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			indexPath := filepath.Join(absPath, "index.html")
			if _, err := os.Stat(indexPath); err != nil {
				// No index.html in directory, serve 404
				serve404(w)
				return
			}
		} else if err != nil {
			// File doesn't exist
			// Check if it's not an API or static asset request
			if !strings.HasPrefix(path, "/api/") &&
				!strings.HasPrefix(path, "/_next/") &&
				!strings.HasPrefix(path, "/static/") &&
				!strings.Contains(path, ".") { // Likely a route, not a file
				serve404(w)
				return
			}
		}

		// File exists or is a static asset, serve it
		fs.ServeHTTP(w, r)
	})
}

// serve404 serves the custom 404.html page
func serve404(w http.ResponseWriter) {
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
