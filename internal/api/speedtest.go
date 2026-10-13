package api

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"connection-check/internal/db"
	"connection-check/internal/models"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow speedtest from configured domains
	},
}

type SpeedTestHandler struct {
	database    *db.DB
	ipLookup    *IPLookupService
	rateLimiter *RateLimiter
	chunkBuffer []byte
	chunkOnce   sync.Once
}

func NewSpeedTestHandler(database *db.DB, ipLookup *IPLookupService, rateLimiter *RateLimiter) *SpeedTestHandler {
	h := &SpeedTestHandler{
		database:    database,
		ipLookup:    ipLookup,
		rateLimiter: rateLimiter,
		chunkBuffer: make([]byte, 1024*1024), // 1MB preallocated buffer
	}
	// Fill with random bytes once to avoid zero-compression artifacts over wire
	h.chunkOnce.Do(func() {
		_, _ = rand.Read(h.chunkBuffer)
	})
	return h
}

// HandleIP returns client IP details, ISP name and current rate-limit quota.
func (h *SpeedTestHandler) HandleIP(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r)
	info, err := h.ipLookup.Lookup(clientIP)
	if err != nil {
		info = &models.IPInfo{IP: clientIP, ISP: "Unknown ISP", Status: "fail"}
	}

	allowed, remaining, resetIn := h.rateLimiter.CheckLimit(clientIP)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ip":           info.IP,
		"isp":          info.ISP,
		"org":          info.Org,
		"city":         info.City,
		"region":       info.Region,
		"country":      info.Country,
		"country_code": info.CountryCode,
		"rate_limit": map[string]interface{}{
			"allowed":      allowed,
			"remaining":    remaining,
			"reset_in_sec": int(resetIn.Seconds()),
			"max_per_hour": 10,
		},
	})
}

// HandlePing handles micro-HTTP latency checks.
func (h *SpeedTestHandler) HandlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	_ = json.NewEncoder(w).Encode(map[string]int64{
		"server_time": time.Now().UnixNano() / 1e6, // ms
	})
}

// HandleWebSocketPing provides high-frequency low-overhead ping & jitter measurements.
func (h *SpeedTestHandler) HandleWebSocketPing(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Echo message back with current server timestamp
		now := time.Now().UnixNano() / 1e6
		resp := map[string]interface{}{
			"echo":        string(p),
			"server_time": now,
		}
		data, _ := json.Marshal(resp)
		if err := conn.WriteMessage(messageType, data); err != nil {
			break
		}
	}
}

// HandleDownload streams chunks of uncompressible data to the client for download testing.
func (h *SpeedTestHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	// Parse requested size, default to 25MB chunks or continuous streaming
	sizeParam := r.URL.Query().Get("size")
	targetSize := int64(25 * 1024 * 1024) // 25MB default
	if sizeParam != "" {
		if s, err := strconv.ParseInt(sizeParam, 10, 64); err == nil && s > 0 && s <= 100*1024*1024 {
			targetSize = s
		}
	}

	w.Header().Set("Content-Length", strconv.FormatInt(targetSize, 10))

	flusher, canFlush := w.(http.Flusher)
	chunkSize := int64(len(h.chunkBuffer))
	var written int64

	for written < targetSize {
		toWrite := targetSize - written
		if toWrite > chunkSize {
			toWrite = chunkSize
		}

		n, err := w.Write(h.chunkBuffer[:toWrite])
		if err != nil {
			return // Client disconnected or finished test early
		}
		written += int64(n)

		if canFlush {
			flusher.Flush()
		}
	}
}

// HandleUpload receives uploaded chunks to measure upload throughput.
func (h *SpeedTestHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Drain incoming body
	bytesRead, err := io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
	if err != nil {
		http.Error(w, "Upload stream read failed", http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()
	if durationMs == 0 {
		durationMs = 1
	}

	mbps := (float64(bytesRead*8) / 1000000.0) / (float64(durationMs) / 1000.0)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"bytes_received": bytesRead,
		"duration_ms":    durationMs,
		"calculated_mbps": mbps,
	})
}

// HandleSaveResult validates rate-limiting, records the test into SQLite, and returns confirmation.
func (h *SpeedTestHandler) HandleSaveResult(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r)

	// Consume rate limit quota (max 10 tests/hour)
	allowed, remaining, resetIn := h.rateLimiter.RecordTest(clientIP)
	if !allowed {
		// Log rate limit violation to bot_logs
		go func() {
			_ = h.database.SaveBotLog(&models.BotLog{
				Timestamp: time.Now().UTC(),
				IP:        clientIP,
				UserAgent: r.UserAgent(),
				Reason:    "Rate limit exceeded (exceeded 10 tests/hour limit)",
				Path:      r.URL.Path,
			})
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":        "Rate limit exceeded: Same IP cannot run more than 10 tests per hour.",
			"reset_in_sec": int(resetIn.Seconds()),
		})
		return
	}

	var req struct {
		DownloadMbps float64 `json:"download_mbps"`
		UploadMbps   float64 `json:"upload_mbps"`
		PingMs       float64 `json:"ping_ms"`
		JitterMs     float64 `json:"jitter_ms"`
		ISP          string  `json:"isp"`
		City         string  `json:"city"`
		Country      string  `json:"country"`
		CountryCode  string  `json:"country_code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Geolocation fallback if client didn't supply ISP
	if req.ISP == "" || req.ISP == "Unknown ISP" {
		if info, err := h.ipLookup.Lookup(clientIP); err == nil {
			req.ISP = info.ISP
			req.City = info.City
			req.Country = info.Country
			req.CountryCode = info.CountryCode
		}
	}

	testRecord := &models.TestResult{
		Timestamp:    time.Now().UTC(),
		IP:           clientIP,
		DownloadMbps: req.DownloadMbps,
		UploadMbps:   req.UploadMbps,
		PingMs:       req.PingMs,
		JitterMs:     req.JitterMs,
		UserAgent:    r.UserAgent(),
		ISP:          req.ISP,
		City:         req.City,
		Country:      req.Country,
		CountryCode:  req.CountryCode,
	}

	if err := h.database.SaveTestResult(testRecord); err != nil {
		http.Error(w, "Failed to save test result", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"id":        testRecord.ID,
		"remaining": remaining,
	})
}
