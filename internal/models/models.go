package models

import "time"

// TestResult represents a completed network speed test record.
type TestResult struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	IP           string    `json:"ip"`
	DownloadMbps float64   `json:"download_mbps"`
	UploadMbps   float64   `json:"upload_mbps"`
	PingMs       float64   `json:"ping_ms"`
	JitterMs     float64   `json:"jitter_ms"`
	UserAgent    string    `json:"user_agent"`
	ISP          string    `json:"isp"`
	City         string    `json:"city"`
	Country      string    `json:"country"`
	CountryCode  string    `json:"country_code"`
}

// BotLog represents a blocked suspicious request or rate limit violation.
type BotLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Reason    string    `json:"reason"`
	Path      string    `json:"path"`
}

// IPInfo holds geolocation and ISP information for a client IP.
type IPInfo struct {
	IP          string `json:"ip"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	AS          string `json:"as"`
	City        string `json:"city"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Status      string `json:"status"`
}

// AdminUser represents the administrator account.
type AdminUser struct {
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	IsDefaultPassword bool      `json:"is_default_password"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DashboardStats contains summary statistics for the admin dashboard.
type DashboardStats struct {
	TotalTests      int64   `json:"total_tests"`
	TestsLast24h    int64   `json:"tests_last_24h"`
	AvgDownloadMbps float64 `json:"avg_download_mbps"`
	AvgUploadMbps   float64 `json:"avg_upload_mbps"`
	AvgPingMs       float64 `json:"avg_ping_ms"`
	AvgJitterMs     float64 `json:"avg_jitter_ms"`
	BlockedBots     int64   `json:"blocked_bots"`
	IsDefaultPass   bool    `json:"is_default_pass"`
}
