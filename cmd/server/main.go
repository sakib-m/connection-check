package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"connection-check/internal/api"
	"connection-check/internal/db"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("PORT", "3000")
	dbPath := getEnv("DB_PATH", "./data/speedtest.db")
	templatesDir := getEnv("TEMPLATES_DIR", "./web/templates")
	staticDir := getEnv("STATIC_DIR", "./web/static")

	log.Printf("[Init] Starting Connection Check & Speedtest Server...")

	// Initialize SQLite Database
	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[DB] Fatal error initializing database at %s: %v", dbPath, err)
	}
	defer database.Close()
	log.Printf("[DB] Database initialized successfully at %s", dbPath)

	// Parse Templates
	pattern := filepath.Join(templatesDir, "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		log.Fatalf("[Templates] Failed to parse HTML templates at %s: %v", pattern, err)
	}
	log.Printf("[Templates] Loaded HTML templates from %s", pattern)

	// Initialize Services
	ipLookup := api.NewIPLookupService()
	rateLimiter := api.NewRateLimiter(10) // 10 tests per hour max per IP
	botGuard := api.NewBotGuard(database)
	authHandler := api.NewAuthHandler(database)
	speedtestHandler := api.NewSpeedTestHandler(database, ipLookup, rateLimiter)
	adminHandler := api.NewAdminHandler(database, authHandler, tmpl)

	// Build Router
	router := api.NewRouter(
		database,
		authHandler,
		botGuard,
		speedtestHandler,
		adminHandler,
		tmpl,
		staticDir,
	)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Server banner
	fmt.Println("==================================================================")
	fmt.Printf("🚀 Connection Check & Speedtest Service running on port %s\n", port)
	fmt.Println("🌐 Public Speedtest UI:  http://localhost:" + port)
	fmt.Println("🔒 Admin Panel URL:      http://localhost:" + port + "/data")
	fmt.Println("🔑 Default Credentials:  admin / admin123 (Password change required on first login)")
	fmt.Println("🛡️  Rate Limit:           10 tests / hour per IP")
	fmt.Println("🤖 Bot Guard:            Active (logs up to 10,000 blocked requests)")
	fmt.Println("==================================================================")

	// Graceful shutdown handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] Failed to listen and serve: %v", err)
		}
	}()

	<-stopChan
	log.Println("[Server] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[Server] Shutdown error: %v", err)
	}
	log.Println("[Server] Server stopped.")
}
