package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"connection-check/internal/models"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// InitDB initializes SQLite database at the specified path and creates schema.
func InitDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer friendly
	conn.SetMaxIdleConns(1)

	database := &DB{conn: conn}
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	if err := database.seedAdmin(); err != nil {
		return nil, fmt.Errorf("failed to seed admin: %w", err)
	}

	return database, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			password_hash TEXT NOT NULL,
			is_default_password INTEGER NOT NULL DEFAULT 1,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS test_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			ip TEXT NOT NULL,
			download_mbps REAL NOT NULL,
			upload_mbps REAL NOT NULL,
			ping_ms REAL NOT NULL,
			jitter_ms REAL NOT NULL,
			user_agent TEXT NOT NULL,
			isp TEXT NOT NULL,
			city TEXT NOT NULL,
			country TEXT NOT NULL,
			country_code TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_test_results_ip ON test_results(ip);`,
		`CREATE INDEX IF NOT EXISTS idx_test_results_timestamp ON test_results(timestamp DESC);`,
		`CREATE TABLE IF NOT EXISTS bot_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			ip TEXT NOT NULL,
			user_agent TEXT NOT NULL,
			reason TEXT NOT NULL,
			path TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_bot_logs_timestamp ON bot_logs(timestamp DESC);`,
	}

	for _, query := range queries {
		if _, err := d.conn.Exec(query); err != nil {
			return fmt.Errorf("migration query error (%s): %w", query, err)
		}
	}

	return nil
}

func (d *DB) seedAdmin() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = d.conn.Exec(
			"INSERT INTO users (username, password_hash, is_default_password, updated_at) VALUES (?, ?, 1, ?)",
			"admin", string(hash), time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		log.Println("[DB] Initialized default admin account (admin / admin123). Password change will be required.")
	}
	return nil
}

// GetUser retrieves user by username.
func (d *DB) GetUser(username string) (*models.AdminUser, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var u models.AdminUser
	var isDefault int
	err := d.conn.QueryRow(
		"SELECT username, password_hash, is_default_password, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&u.Username, &u.PasswordHash, &isDefault, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.IsDefaultPassword = (isDefault == 1)
	return &u, nil
}

// UpdatePassword updates admin password and marks is_default_password as 0.
func (d *DB) UpdatePassword(username, newPassword string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec(
		"UPDATE users SET password_hash = ?, is_default_password = 0, updated_at = ? WHERE username = ?",
		string(hash), time.Now().UTC(), username,
	)
	return err
}

// SaveTestResult inserts a completed test result into database.
func (d *DB) SaveTestResult(res *models.TestResult) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if res.Timestamp.IsZero() {
		res.Timestamp = time.Now().UTC()
	}

	stmt, err := d.conn.Prepare(`
		INSERT INTO test_results (timestamp, ip, download_mbps, upload_mbps, ping_ms, jitter_ms, user_agent, isp, city, country, country_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	r, err := stmt.Exec(
		res.Timestamp, res.IP, res.DownloadMbps, res.UploadMbps,
		res.PingMs, res.JitterMs, res.UserAgent, res.ISP,
		res.City, res.Country, res.CountryCode,
	)
	if err != nil {
		return err
	}

	id, err := r.LastInsertId()
	if err == nil {
		res.ID = id
	}
	return nil
}

// GetTestResults retrieves paginated list of test results.
func (d *DB) GetTestResults(limit, offset int, search string) ([]models.TestResult, int64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var total int64
	var queryCount, querySelect string
	var argsCount, argsSelect []interface{}

	if search != "" {
		s := "%" + search + "%"
		queryCount = "SELECT COUNT(*) FROM test_results WHERE ip LIKE ? OR isp LIKE ? OR city LIKE ? OR country LIKE ?"
		argsCount = []interface{}{s, s, s, s}

		querySelect = `SELECT id, timestamp, ip, download_mbps, upload_mbps, ping_ms, jitter_ms, user_agent, isp, city, country, country_code 
		               FROM test_results WHERE ip LIKE ? OR isp LIKE ? OR city LIKE ? OR country LIKE ? 
		               ORDER BY id DESC LIMIT ? OFFSET ?`
		argsSelect = []interface{}{s, s, s, s, limit, offset}
	} else {
		queryCount = "SELECT COUNT(*) FROM test_results"
		querySelect = `SELECT id, timestamp, ip, download_mbps, upload_mbps, ping_ms, jitter_ms, user_agent, isp, city, country, country_code 
		               FROM test_results ORDER BY id DESC LIMIT ? OFFSET ?`
		argsSelect = []interface{}{limit, offset}
	}

	if err := d.conn.QueryRow(queryCount, argsCount...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := d.conn.Query(querySelect, argsSelect...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []models.TestResult
	for rows.Next() {
		var item models.TestResult
		if err := rows.Scan(
			&item.ID, &item.Timestamp, &item.IP, &item.DownloadMbps, &item.UploadMbps,
			&item.PingMs, &item.JitterMs, &item.UserAgent, &item.ISP,
			&item.City, &item.Country, &item.CountryCode,
		); err != nil {
			return nil, 0, err
		}
		results = append(results, item)
	}

	return results, total, nil
}

// SaveBotLog logs a blocked request and enforces maximum 10,000 rows.
func (d *DB) SaveBotLog(logItem *models.BotLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if logItem.Timestamp.IsZero() {
		logItem.Timestamp = time.Now().UTC()
	}

	_, err := d.conn.Exec(`
		INSERT INTO bot_logs (timestamp, ip, user_agent, reason, path)
		VALUES (?, ?, ?, ?, ?)
	`, logItem.Timestamp, logItem.IP, logItem.UserAgent, logItem.Reason, logItem.Path)
	if err != nil {
		return err
	}

	// Prune older rows beyond 10,000
	_, _ = d.conn.Exec(`
		DELETE FROM bot_logs 
		WHERE id NOT IN (
			SELECT id FROM bot_logs ORDER BY id DESC LIMIT 10000
		)
	`)

	return nil
}

// GetBotLogs retrieves paginated list of blocked bot records up to 10,000 rows.
func (d *DB) GetBotLogs(limit, offset int, search string) ([]models.BotLog, int64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var total int64
	var queryCount, querySelect string
	var argsCount, argsSelect []interface{}

	if search != "" {
		s := "%" + search + "%"
		queryCount = "SELECT COUNT(*) FROM bot_logs WHERE ip LIKE ? OR user_agent LIKE ? OR reason LIKE ? OR path LIKE ?"
		argsCount = []interface{}{s, s, s, s}

		querySelect = `SELECT id, timestamp, ip, user_agent, reason, path 
		               FROM bot_logs WHERE ip LIKE ? OR user_agent LIKE ? OR reason LIKE ? OR path LIKE ? 
		               ORDER BY id DESC LIMIT ? OFFSET ?`
		argsSelect = []interface{}{s, s, s, s, limit, offset}
	} else {
		queryCount = "SELECT COUNT(*) FROM bot_logs"
		querySelect = `SELECT id, timestamp, ip, user_agent, reason, path 
		               FROM bot_logs ORDER BY id DESC LIMIT ? OFFSET ?`
		argsSelect = []interface{}{limit, offset}
	}

	if err := d.conn.QueryRow(queryCount, argsCount...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := d.conn.Query(querySelect, argsSelect...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.BotLog
	for rows.Next() {
		var item models.BotLog
		if err := rows.Scan(&item.ID, &item.Timestamp, &item.IP, &item.UserAgent, &item.Reason, &item.Path); err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}

	return logs, total, nil
}

// ClearBotLogs purges the bot log table.
func (d *DB) ClearBotLogs() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("DELETE FROM bot_logs")
	return err
}

// GetDashboardStats aggregates live metrics.
func (d *DB) GetDashboardStats() (*models.DashboardStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &models.DashboardStats{}

	// Total tests & averages
	_ = d.conn.QueryRow(`
		SELECT COUNT(*), 
		       COALESCE(AVG(download_mbps), 0), 
		       COALESCE(AVG(upload_mbps), 0), 
		       COALESCE(AVG(ping_ms), 0), 
		       COALESCE(AVG(jitter_ms), 0)
		FROM test_results
	`).Scan(&stats.TotalTests, &stats.AvgDownloadMbps, &stats.AvgUploadMbps, &stats.AvgPingMs, &stats.AvgJitterMs)

	// Tests in last 24h
	cutoff24h := time.Now().UTC().Add(-24 * time.Hour)
	_ = d.conn.QueryRow(`
		SELECT COUNT(*) FROM test_results WHERE timestamp >= ?
	`, cutoff24h).Scan(&stats.TestsLast24h)

	// Blocked bots
	_ = d.conn.QueryRow(`SELECT COUNT(*) FROM bot_logs`).Scan(&stats.BlockedBots)

	// Admin user default password status
	var isDefault int
	_ = d.conn.QueryRow(`SELECT is_default_password FROM users WHERE username = 'admin'`).Scan(&isDefault)
	stats.IsDefaultPass = (isDefault == 1)

	return stats, nil
}
