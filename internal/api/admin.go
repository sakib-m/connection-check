package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"connection-check/internal/db"
)

type AdminHandler struct {
	database   *db.DB
	auth       *AuthHandler
	sslManager *SSLManager
	templates  *template.Template
}

func NewAdminHandler(database *db.DB, auth *AuthHandler, sslManager *SSLManager, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{
		database:   database,
		auth:       auth,
		sslManager: sslManager,
		templates:  tmpl,
	}
}

// ServeLoginPage renders the admin login screen.
func (a *AdminHandler) ServeLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if _, ok := a.auth.GetSessionFromRequest(r); ok {
		http.Redirect(w, r, "/data", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = a.templates.ExecuteTemplate(w, "login.html", nil)
}

// ServeDashboardPage renders the main admin dashboard screen.
func (a *AdminHandler) ServeDashboardPage(w http.ResponseWriter, r *http.Request) {
	session, ok := a.auth.GetSessionFromRequest(r)
	if !ok {
		http.Redirect(w, r, "/data/login", http.StatusSeeOther)
		return
	}

	user, err := a.database.GetUser(session.Username)
	if err != nil {
		http.Redirect(w, r, "/data/login", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"Username":          user.Username,
		"IsDefaultPassword": user.IsDefaultPassword,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = a.templates.ExecuteTemplate(w, "admin.html", data)
}

// GetStats returns aggregated statistics.
func (a *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.database.GetDashboardStats()
	if err != nil {
		http.Error(w, "Failed to retrieve stats", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// GetTests returns paginated test logs.
func (a *AdminHandler) GetTests(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}
	search := r.URL.Query().Get("search")
	offset := (page - 1) * limit

	results, total, err := a.database.GetTestResults(limit, offset, search)
	if err != nil {
		http.Error(w, "Failed to retrieve test logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// ExportTestsCSV exports tests logs to downloadable CSV file.
func (a *AdminHandler) ExportTestsCSV(w http.ResponseWriter, r *http.Request) {
	results, _, err := a.database.GetTestResults(10000, 0, "")
	if err != nil {
		http.Error(w, "Failed to export records", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("speedtest_records_%s.csv", time.Now().Format("2006-01-02_150405"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename="+filename)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write CSV Header
	_ = writer.Write([]string{"ID", "Timestamp (UTC)", "IP", "Download (Mbps)", "Upload (Mbps)", "Ping (ms)", "Jitter (ms)", "ISP", "City", "Country", "User Agent"})

	for _, item := range results {
		row := []string{
			strconv.FormatInt(item.ID, 10),
			item.Timestamp.Format(time.RFC3339),
			item.IP,
			fmt.Sprintf("%.2f", item.DownloadMbps),
			fmt.Sprintf("%.2f", item.UploadMbps),
			fmt.Sprintf("%.2f", item.PingMs),
			fmt.Sprintf("%.2f", item.JitterMs),
			item.ISP,
			item.City,
			item.Country,
			item.UserAgent,
		}
		_ = writer.Write(row)
	}
}

// GetBotLogs returns paginated blocked bot attempts (up to 10,000 rows).
func (a *AdminHandler) GetBotLogs(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 25
	}
	search := r.URL.Query().Get("search")
	offset := (page - 1) * limit

	logs, total, err := a.database.GetBotLogs(limit, offset, search)
	if err != nil {
		http.Error(w, "Failed to retrieve bot logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ClearBotLogs empties the blocked bots log table.
func (a *AdminHandler) ClearBotLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.database.ClearBotLogs(); err != nil {
		http.Error(w, "Failed to clear logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// SaveDomain saves the FQDN setting.
func (a *AdminHandler) SaveDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FQDN string `json:"fqdn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := a.sslManager.ValidateDomain(req.FQDN); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := a.database.SetSetting("fqdn", req.FQDN); err != nil {
		http.Error(w, "Failed to save domain", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"fqdn":    req.FQDN,
	})
}

// VerifyDNSAndSSL verifies DNS resolution and triggers Let's Encrypt SSL issuance.
func (a *AdminHandler) VerifyDNSAndSSL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FQDN string `json:"fqdn"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetFQDN := req.FQDN
	if targetFQDN == "" {
		savedFQDN, err := a.database.GetSetting("fqdn")
		if err != nil || savedFQDN == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "FQDN is not specified. Please configure your domain name first.",
			})
			return
		}
		targetFQDN = savedFQDN
	}

	result, _ := a.sslManager.VerifyDNSAndIssueSSL(targetFQDN)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
