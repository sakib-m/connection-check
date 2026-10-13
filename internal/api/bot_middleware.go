package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"connection-check/internal/db"
	"connection-check/internal/models"
)

type BotGuard struct {
	database *db.DB
}

func NewBotGuard(database *db.DB) *BotGuard {
	return &BotGuard{database: database}
}

// IsBot evaluates whether the User-Agent is automated, scraping, or not a real browser.
func (bg *BotGuard) IsBot(ua string) (bool, string) {
	trimmed := strings.TrimSpace(ua)
	if trimmed == "" {
		return true, "Missing User-Agent header"
	}
	if len(trimmed) < 15 {
		return true, "Suspiciously short User-Agent"
	}

	lower := strings.ToLower(trimmed)

	// Block known CLI tools, libraries, bots and scanners
	blockedTokens := []string{
		"curl/", "wget/", "python-requests", "python-urllib", "aiohttp", "httpx",
		"scrapy", "headlesschrome", "phantomjs", "postmanruntime", "insomnia",
		"httpclient", "go-http-client", "apache-httpclient", "okhttp", "java/",
		"bot", "spider", "crawl", "zgrab", "nmap", "masscan", "nikto", "sqlmap",
		"censys", "shodan", "semrush", "ahrefs", "bytespider", "petalbot",
		"yandexbot", "bingbot", "googlebot", "duckduckbot", "baiduspider", "slurp",
		"curl", "wget", "node-fetch", "undici", "axios",
	}

	for _, token := range blockedTokens {
		if strings.Contains(lower, token) {
			return true, "Known bot/crawler/CLI agent (" + token + ")"
		}
	}

	// Legitimate browsers generally contain "Mozilla/" and an engine token
	if !strings.Contains(trimmed, "Mozilla/") {
		return true, "Missing standard browser identifier (Mozilla/)"
	}

	hasBrowserEngine := strings.Contains(lower, "applewebkit") ||
		strings.Contains(lower, "gecko") ||
		strings.Contains(lower, "chrome") ||
		strings.Contains(lower, "safari") ||
		strings.Contains(lower, "firefox") ||
		strings.Contains(lower, "edg") ||
		strings.Contains(lower, "opera") ||
		strings.Contains(lower, "opr")

	if !hasBrowserEngine {
		return true, "Unrecognized browser engine"
	}

	return false, ""
}

// Middleware returns an HTTP handler that filters out bot requests and logs them.
func (bg *BotGuard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.UserAgent()
		isBot, reason := bg.IsBot(ua)
		if isBot {
			clientIP := GetClientIP(r)
			// Log to database asynchronously
			go func() {
				_ = bg.database.SaveBotLog(&models.BotLog{
					Timestamp: time.Now().UTC(),
					IP:        clientIP,
					UserAgent: ua,
					Reason:    reason,
					Path:      r.URL.Path,
				})
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Access denied: Bot or automated scraper detected.",
				"reason":  reason,
				"blocked": true,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
