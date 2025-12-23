package api

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/k8s-lynq/lynq/dashboard/bff/internal/kube"
)

func init() {
	// Register common MIME types (Alpine Linux doesn't have mime.types)
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".mjs", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".html", "text/html")
	mime.AddExtensionType(".json", "application/json")
	mime.AddExtensionType(".png", "image/png")
	mime.AddExtensionType(".jpg", "image/jpeg")
	mime.AddExtensionType(".jpeg", "image/jpeg")
	mime.AddExtensionType(".gif", "image/gif")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".ico", "image/x-icon")
	mime.AddExtensionType(".woff", "font/woff")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".ttf", "font/ttf")
	mime.AddExtensionType(".eot", "application/vnd.ms-fontobject")
	mime.AddExtensionType(".map", "application/json")
}

// NewRouter creates a new HTTP router
func NewRouter(kubeClient *kube.Client, appMode string) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Create handler
	h := NewHandler(kubeClient, appMode)

	// Health endpoints
	r.Get("/healthz", h.HealthCheck)
	r.Get("/readyz", h.ReadyCheck)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Hub routes
		r.Route("/hubs", func(r chi.Router) {
			r.Get("/", h.ListHubs)
			r.Get("/{name}", h.GetHub)
			r.Get("/{name}/nodes", h.GetHubNodes)
		})

		// Form routes
		r.Route("/forms", func(r chi.Router) {
			r.Get("/", h.ListForms)
			r.Get("/{name}", h.GetForm)
			r.Get("/{name}/details", h.GetFormDetails)
		})

		// Node routes
		r.Route("/nodes", func(r chi.Router) {
			r.Get("/", h.ListNodes)
			r.Get("/{name}", h.GetNode)
			r.Get("/{name}/resources", h.GetNodeResources)
			r.Get("/{name}/events", h.GetNodeEvents)
		})

		// Topology
		r.Get("/topology", h.GetTopology)

		// Resource (generic K8s resource fetch)
		r.Get("/resources", h.GetResource)

		// Events (generic events by name/namespace)
		r.Get("/events", h.GetEvents)

		// Context routes (local mode only)
		r.Route("/contexts", func(r chi.Router) {
			r.Get("/", h.ListContexts)
			r.Post("/switch", h.SwitchContext)
		})
	})

	// Serve static files (SPA support)
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./public"
	}
	if _, err := os.Stat(staticDir); err == nil {
		r.Get("/*", spaHandler(staticDir))
	}

	return r
}

// spaHandler serves static files and falls back to index.html for SPA routing
func spaHandler(staticDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))

	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := filepath.Clean(r.URL.Path)
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists
		fullPath := filepath.Join(staticDir, path)
		info, err := os.Stat(fullPath)

		// If file doesn't exist or is a directory without index.html, serve index.html (SPA fallback)
		if os.IsNotExist(err) || (info != nil && info.IsDir()) {
			// Check if it's an API route that wasn't matched (shouldn't happen, but safety check)
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			// Serve index.html for SPA routing
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set cache headers for static assets
		if isStaticAsset(path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		fileServer.ServeHTTP(w, r)
	}
}

// isStaticAsset checks if the path is a static asset that can be cached
func isStaticAsset(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	staticExts := map[string]bool{
		".js": true, ".css": true, ".png": true, ".jpg": true,
		".jpeg": true, ".gif": true, ".svg": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	}
	return staticExts[ext]
}
