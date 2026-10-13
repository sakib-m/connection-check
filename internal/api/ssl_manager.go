package api

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"connection-check/internal/db"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

type SSLManager struct {
	database    *db.DB
	caddyAPIURL string
}

func NewSSLManager(database *db.DB) *SSLManager {
	apiURL := os.Getenv("CADDY_ADMIN_URL")
	if apiURL == "" {
		apiURL = "http://caddy:2019"
	}
	return &SSLManager{
		database:    database,
		caddyAPIURL: strings.TrimRight(apiURL, "/"),
	}
}

type SSLVerificationResult struct {
	Success      bool     `json:"success"`
	FQDN         string   `json:"fqdn"`
	ResolvedIPs  []string `json:"resolved_ips"`
	Message      string   `json:"message"`
	CaddyUpdated bool     `json:"caddy_updated"`
	Status       string   `json:"status"`
}

// ValidateDomain validates domain string syntax.
func (s *SSLManager) ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain name cannot be empty")
	}
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain name format (e.g. speed.example.com)")
	}
	return nil
}

// VerifyDNSAndIssueSSL checks DNS resolution and triggers Caddy Let's Encrypt certificate acquisition.
func (s *SSLManager) VerifyDNSAndIssueSSL(fqdn string) (*SSLVerificationResult, error) {
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	if err := s.ValidateDomain(fqdn); err != nil {
		return &SSLVerificationResult{
			Success: false,
			FQDN:    fqdn,
			Message: err.Error(),
			Status:  "invalid_domain",
		}, err
	}

	// 1. Check DNS resolution
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var resolver net.Resolver
	ips, err := resolver.LookupIP(ctx, "ip", fqdn)
	if err != nil || len(ips) == 0 {
		msg := fmt.Sprintf("DNS resolution failed for '%s'. Please ensure you added an 'A' or 'AAAA' record in your DNS control panel pointing to this server's public IP address.", fqdn)
		_ = s.database.SetSetting("ssl_status", "dns_lookup_failed")
		return &SSLVerificationResult{
			Success: false,
			FQDN:    fqdn,
			Message: msg,
			Status:  "dns_not_resolved",
		}, nil
	}

	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	// Save FQDN to settings
	_ = s.database.SetSetting("fqdn", fqdn)
	_ = s.database.SetSetting("last_ssl_check", time.Now().UTC().Format(time.RFC3339))

	// 2. Configure Caddy via dynamic Caddyfile or Admin API
	caddyUpdated := false
	caddyErr := s.updateCaddyConfig(fqdn)
	if caddyErr != nil {
		statusMsg := fmt.Sprintf("DNS resolved to [%s]. Warning: Could not communicate with Caddy Admin API (%v). If running behind external proxy, ensure port 80/443 points to Caddy.", strings.Join(ipStrings, ", "), caddyErr)
		_ = s.database.SetSetting("ssl_status", "caddy_api_unreachable")
		return &SSLVerificationResult{
			Success:      true,
			FQDN:         fqdn,
			ResolvedIPs:  ipStrings,
			Message:      statusMsg,
			CaddyUpdated: false,
			Status:       "dns_ok_caddy_pending",
		}, nil
	}

	caddyUpdated = true
	_ = s.database.SetSetting("ssl_status", "active")
	_ = s.database.SetSetting("ssl_enabled", "true")

	successMsg := fmt.Sprintf("DNS verified successfully! '%s' resolves to [%s]. Caddy has been instructed to automatically provision and renew Let's Encrypt SSL certificate over ports 80 and 443.", fqdn, strings.Join(ipStrings, ", "))

	return &SSLVerificationResult{
		Success:      true,
		FQDN:         fqdn,
		ResolvedIPs:  ipStrings,
		Message:      successMsg,
		CaddyUpdated: caddyUpdated,
		Status:       "active",
	}, nil
}

// updateCaddyConfig posts the new configuration to Caddy's dynamic admin API (/load).
func (s *SSLManager) updateCaddyConfig(fqdn string) error {
	// Construct Caddyfile configuration dynamically for Caddy
	// This instructs Caddy to listen on fqdn on 80/443 with automated Let's Encrypt SSL,
	// plus default fallback on :80 and proxy to app:3000
	caddyfileContent := fmt.Sprintf(`{
    admin 0.0.0.0:2019
}

# Configured FQDN with automated Let's Encrypt SSL
%s {
    reverse_proxy app:3000 {
        header_up Host {host}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }
}

# Fallback HTTP access for direct IP connections
:80 {
    reverse_proxy app:3000 {
        header_up Host {host}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }
}
`, fqdn)

	// Send POST /load with Caddyfile body to Caddy admin API
	req, err := http.NewRequest(http.MethodPost, s.caddyAPIURL+"/load", bytes.NewBufferString(caddyfileContent))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("caddy responded with status code %d", resp.StatusCode)
	}

	return nil
}
