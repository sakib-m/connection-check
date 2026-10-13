package api

import (
	"html/template"
	"net/http"
	"strings"

	"connection-check/internal/db"
)

type Router struct {
	mux        *http.ServeMux
	db         *db.DB
	auth       *AuthHandler
	botGuard   *BotGuard
	speedtest  *SpeedTestHandler
	admin      *AdminHandler
	templates  *template.Template
	staticPath string
}

func NewRouter(
	database *db.DB,
	auth *AuthHandler,
	botGuard *BotGuard,
	speedtest *SpeedTestHandler,
	admin *AdminHandler,
	tmpl *template.Template,
	staticDir string,
) *Router {
	r := &Router{
		mux:        http.NewServeMux(),
		db:         database,
		auth:       auth,
		botGuard:   botGuard,
		speedtest:  speedtest,
		admin:      admin,
		templates:  tmpl,
		staticPath: staticDir,
	}

	r.setupRoutes()
	return r
}

func (r *Router) setupRoutes() {
	// Static files handler
	fs := http.FileServer(http.Dir(r.staticPath))
	r.mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Frontend main page
	r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = r.templates.ExecuteTemplate(w, "index.html", nil)
	})

	// Speedtest API endpoints (wrapped in BotGuard middleware)
	botProtected := r.botGuard.Middleware

	// IP info & quota
	r.mux.Handle("/api/speedtest/ip", botProtected(http.HandlerFunc(r.speedtest.HandleIP)))

	// Ping & Jitter
	r.mux.Handle("/api/speedtest/ping", botProtected(http.HandlerFunc(r.speedtest.HandlePing)))
	r.mux.Handle("/api/speedtest/ws", http.HandlerFunc(r.speedtest.HandleWebSocketPing))

	// Download & Upload tests
	r.mux.Handle("/api/speedtest/download", botProtected(http.HandlerFunc(r.speedtest.HandleDownload)))
	r.mux.Handle("/api/speedtest/upload", botProtected(http.HandlerFunc(r.speedtest.HandleUpload)))

	// Save test result (also rate limited)
	r.mux.Handle("/api/speedtest/results", botProtected(http.HandlerFunc(r.speedtest.HandleSaveResult)))

	// Admin Panel UI
	r.mux.HandleFunc("/data", r.admin.ServeDashboardPage)
	r.mux.HandleFunc("/data/login", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			r.auth.HandleLogin(w, req)
			return
		}
		r.admin.ServeLoginPage(w, req)
	})
	r.mux.HandleFunc("/data/logout", r.auth.HandleLogout)

	// Admin APIs (protected by Session Auth)
	r.mux.HandleFunc("/data/api/stats", r.auth.RequireAuth(r.admin.GetStats))
	r.mux.HandleFunc("/data/api/tests", r.auth.RequireAuth(r.admin.GetTests))
	r.mux.HandleFunc("/data/api/export-tests", r.auth.RequireAuth(r.admin.ExportTestsCSV))
	r.mux.HandleFunc("/data/api/bots", r.auth.RequireAuth(r.admin.GetBotLogs))
	r.mux.HandleFunc("/data/api/clear-bots", r.auth.RequireAuth(r.admin.ClearBotLogs))
	r.mux.HandleFunc("/data/api/change-password", r.auth.RequireAuth(r.auth.HandleChangePassword))
}

// ServeHTTP implements http.Handler with global CORS and basic security headers.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	if strings.HasPrefix(req.URL.Path, "/api/speedtest") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, User-Agent")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	r.mux.ServeHTTP(w, req)
}
