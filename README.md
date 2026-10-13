# ⚡ SpeedTest Pro & Connection Check

A modern, high-throughput network connection diagnostic and speedtest suite written in **Go (Golang)** with an automated **Caddy Let's Encrypt reverse proxy**, an **Admin Control Panel (`/data`)**, comprehensive SQLite test analytics, IP & ISP detection, real-time gauges & charts, strict **IP rate limiting**, and **Bot / Scraper Guard**.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://docker.com)
[![Reverse Proxy](https://img.shields.io/badge/SSL-Let's%20Encrypt%20via%20Caddy-00D3A9?style=flat&logo=caddy)](https://caddyserver.com)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind-CSS%203-38B2AC?style=flat&logo=tailwind-css)](https://tailwindcss.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 Key Features

### 🚀 Public Speedtest Diagnostic
- **Accurate Throughput Testing**: Multi-stream adaptive chunk streaming in pure Go for accurate Gigabit / Fiber testing without CPU bloat.
- **Micro-second Latency & Jitter**: High-frequency ping round-trips with RFC 3550 jitter calculation.
- **Client IP & ISP Intelligence**: Identifies client's public IP address, ISP name, City, Region, Country, and ASN with server-side 24-hour caching.
- **Modern Cyberpunk / Clean Dark UI**: Built with Tailwind CSS, animated HUD cards, interactive canvas speedometer gauge, and real-time Chart.js spline curves.
- **Responsive Design**: Flawless experience across desktop, tablet, and mobile devices.

### 🛡️ Rate Limiting & Bot Protection
- **Hourly IP Rate Limit**: Restricts test executions to **10 tests per single hour per IP address** using a sliding-window tracker.
- **Automated Bot Guard**: Blocks automated scrapers, headless crawlers, CLI tools (`curl`, `wget`, `python-requests`, etc.), and invalid User-Agents.
- **Blocked Bots Log**: Logs suspicious and blocked attempts to an admin menu capped at **10,000 rows** (FIFO rotation).

### 🔒 Admin Panel (`/data`)
- **Default Authentication**: Initial credentials are `admin` / `admin123`.
- **First Initialization Security**: Mandatory password change dialog prevents unauthorized access before configuring a strong custom password.
- **Domain & Let's Encrypt SSL Manager**:
  - Configure your custom domain / FQDN (e.g. `speed.yourdomain.com`).
  - **"Verify DNS & Generate Let's Encrypt SSL" Button**: Performs server-side DNS resolution checks and dynamically commands the Caddy reverse proxy to acquire and renew SSL certificates on ports `80` and `443`.
- **Comprehensive Test Logs Table**:
  - Columns: `Timestamp (UTC)`, `Client IP`, `Download (Mbps)`, `Upload (Mbps)`, `Ping (ms)`, `Jitter (ms)`, `ISP`, `Location`, `User Agent`.
  - Live search, pagination, and **one-click CSV export**.
- **Blocked Bots Inspection Menu**: Paginated view of blocked attempts with reason, request path, source IP, and User-Agent.

---

## 🏗️ Architecture

```
Internet (Ports 80 & 443)
           │
           ▼
┌──────────────────────────────────────────────┐
│       Caddy Reverse Proxy (Container)        │
│  - Automated Let's Encrypt TLS (80 / 443)    │
│  - HTTP/3 QUIC enabled                       │
│  - Dynamic Admin API on port 2019            │
└──────────────────────┬───────────────────────┘
                       │ Internal Docker Network
                       ▼
┌──────────────────────────────────────────────┐
│          Go Speedtest App (Container)        │
│  - Port 3000 (Internal)                      │
│  - Low-latency Speedtest Engine              │
│  - Bot Guard & 10/hour IP Rate Limiter       │
│  - Admin Console at /data                    │
│  - Persistent SQLite Database (data/)        │
└──────────────────────────────────────────────┘
```

---

## 🚀 Quick Start with Docker Compose

### 1. Clone the repository
```bash
git clone https://github.com/your-username/connection-check.git
cd connection-check
```

### 2. Launch with Docker Compose
```bash
docker compose up -d --build
```

The services will start automatically:
- **Public Speedtest UI**: `http://localhost` (or `http://YOUR_SERVER_IP`)
- **Admin Panel**: `http://localhost/data` (or `http://YOUR_SERVER_IP/data`)
- **Direct App Port (Optional)**: `http://localhost:3000`

---

## 🌐 Configuring Custom FQDN & Let's Encrypt SSL

1. **Point your DNS Record**:
   - In your DNS provider (Cloudflare, Namecheap, Route53, GoDaddy, etc.), create an **`A` record**:
     - **Name**: `speed` (or `@` for root domain)
     - **Value**: Your server's public IPv4 address
     - **TTL**: Auto / 5 minutes
2. **Access the Admin Panel**:
   - Navigate to `http://YOUR_SERVER_IP/data`
   - Log in with default credentials: `admin` / `admin123`
   - Change your password when prompted on first initialization.
3. **Configure FQDN & Generate SSL**:
   - Click on the **"Domain & SSL (FQDN)"** tab.
   - Enter your domain name (e.g., `speed.yourdomain.com`).
   - Click **"Verify DNS & Generate Let's Encrypt SSL"**.
   - The backend checks if the domain resolves to your server, communicates with the Caddy proxy, and automatically requests the TLS certificate from Let's Encrypt.
   - Your speedtest website is now live over HTTPS: `https://speed.yourdomain.com`!

---

## 🔑 Default Credentials

| Field | Value |
|---|---|
| **Admin Route** | `/data` |
| **Username** | `admin` |
| **Default Password** | `admin123` |
| **First Login** | Enforces instant password change |

---

## ⚙️ Environment Variables

The application can be configured via environment variables in `docker-compose.yml` or a `.env` file:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3000` | Port the internal Go application listens on |
| `DB_PATH` | `/app/data/speedtest.db` | Path to persistent SQLite database file |
| `TEMPLATES_DIR` | `/app/web/templates` | Directory containing HTML templates |
| `STATIC_DIR` | `/app/web/static` | Directory containing static CSS and JS |
| `CADDY_ADMIN_URL`| `http://caddy:2019` | Caddy Admin API endpoint for dynamic SSL provisioning |

---

## 🛠️ API Reference

### Public Diagnostics API
- `GET /api/speedtest/ip`: Returns client IP, ISP, location, and remaining hourly quota.
- `GET /api/speedtest/ping`: Returns high-precision server timestamp for latency & jitter.
- `GET /api/speedtest/ws`: WebSocket endpoint for continuous real-time ping-pong measurements.
- `GET /api/speedtest/download?size=30000000`: Streams uncompressible binary data for download measurement.
- `POST /api/speedtest/upload`: Receives streamed payload to measure upload throughput.
- `POST /api/speedtest/results`: Saves completed test result (enforces 10 tests/hour rate limit).

### Protected Admin API (`/data/api/*`)
- `GET /data/api/stats`: Summary metrics (totals, 24h count, averages, blocked bots, SSL status).
- `GET /data/api/tests?page=1&limit=25&search=...`: Paginated speedtest logs.
- `GET /data/api/export-tests`: Downloads speedtest logs as CSV.
- `GET /data/api/bots?page=1&limit=25&search=...`: Paginated blocked bot logs (capped at 10,000 rows).
- `POST /data/api/clear-bots`: Clears bot log history.
- `POST /data/api/settings/domain`: Sets FQDN.
- `POST /data/api/settings/verify-ssl`: Runs DNS verification and triggers Caddy Let's Encrypt certificate acquisition.
- `POST /data/api/change-password`: Updates admin credentials and marks default password as changed.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
