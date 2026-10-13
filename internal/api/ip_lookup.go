package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"connection-check/internal/models"
)

type cachedIPInfo struct {
	info      *models.IPInfo
	expiresAt time.Time
}

type IPLookupService struct {
	cache sync.Map
}

func NewIPLookupService() *IPLookupService {
	s := &IPLookupService{}
	// Periodic cleanup of expired cache items every 30 minutes
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			now := time.Now()
			s.cache.Range(func(key, value interface{}) bool {
				if item, ok := value.(cachedIPInfo); ok {
					if now.After(item.expiresAt) {
						s.cache.Delete(key)
					}
				}
				return true
			})
		}
	}()
	return s
}

// GetClientIP resolves the actual public IP address from standard proxy headers or RemoteAddr.
func GetClientIP(r *http.Request) string {
	// Cloudflare
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}

	// Standard reverse proxies (Caddy, Nginx)
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// X-Forwarded-For chain (take the leftmost client IP)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// isPrivateIP checks if the IP address is loopback, link-local or RFC1918 private.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// Lookup queries geolocation and ISP information for the given IP with server caching.
func (s *IPLookupService) Lookup(clientIP string) (*models.IPInfo, error) {
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}

	// Check cache (TTL 24 hours)
	if val, ok := s.cache.Load(clientIP); ok {
		item := val.(cachedIPInfo)
		if time.Now().Before(item.expiresAt) {
			return item.info, nil
		}
	}

	// If local/private IP
	if isPrivateIP(clientIP) {
		info := &models.IPInfo{
			IP:          clientIP,
			ISP:         "Local / Private Network",
			Org:         "Local Intranet",
			AS:          "AS0 Private",
			City:        "Internal",
			Region:      "Local",
			Country:     "Localhost",
			CountryCode: "LOC",
			Status:      "success",
		}
		s.cache.Store(clientIP, cachedIPInfo{info: info, expiresAt: time.Now().Add(24 * time.Hour)})
		return info, nil
	}

	// Query ip-api.com
	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,region,regionName,city,isp,org,as,query", clientIP)
	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		// Fallback minimal info
		return &models.IPInfo{
			IP:          clientIP,
			ISP:         "Unknown ISP",
			City:        "Unknown",
			Country:     "Unknown",
			CountryCode: "UN",
			Status:      "fail",
		}, nil
	}
	defer resp.Body.Close()

	var apiResp struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		Region      string `json:"region"`
		RegionName  string `json:"regionName"`
		City        string `json:"city"`
		ISP         string `json:"isp"`
		Org         string `json:"org"`
		AS          string `json:"as"`
		Query       string `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil || apiResp.Status != "success" {
		info := &models.IPInfo{
			IP:          clientIP,
			ISP:         "Public Network",
			City:        "Unknown",
			Country:     "Unknown",
			CountryCode: "UN",
			Status:      "fail",
		}
		return info, nil
	}

	ispName := apiResp.ISP
	if ispName == "" {
		ispName = apiResp.Org
	}
	if ispName == "" {
		ispName = "Internet Service Provider"
	}

	info := &models.IPInfo{
		IP:          clientIP,
		ISP:         ispName,
		Org:         apiResp.Org,
		AS:          apiResp.AS,
		City:        apiResp.City,
		Region:      apiResp.RegionName,
		Country:     apiResp.Country,
		CountryCode: apiResp.CountryCode,
		Status:      "success",
	}

	s.cache.Store(clientIP, cachedIPInfo{
		info:      info,
		expiresAt: time.Now().Add(24 * time.Hour),
	})

	return info, nil
}
