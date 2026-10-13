# ⚡ SpeedTest Pro & Connection Check

A modern, high-throughput network connection diagnostic and speedtest suite written in **Go (Golang)** with an **Admin Control Panel (`/data`)**, comprehensive SQLite test analytics, IP & ISP detection, real-time gauges & charts, strict **IP rate limiting**, and **Bot / Scraper Guard**.

Designed to run as a lightweight container on **port 3000** behind your preferred reverse proxy (**Nginx**, **Apache**, **Caddy**, or **Traefik**).

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://docker.com)
[![Port](https://img.shields.io/badge/Port-3000-blue)](http://localhost:3000)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind-CSS%203-38B2AC?style=flat&logo=tailwind-css)](https://tailwindcss.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 Key Features

### 🚀 Public Speedtest Diagnostic
- **Accurate Throughput Testing**: Multi-stream adaptive chunk streaming in pure Go for accurate Gigabit / Fiber testing without CPU spikes.
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
- **Comprehensive Test Logs Table**:
  - Columns: `Timestamp (UTC)`, `Client IP`, `Download (Mbps)`, `Upload (Mbps)`, `Ping (ms)`, `Jitter (ms)`, `ISP`, `Location`, `User Agent`.
  - Live search, pagination, and **one-click CSV export**.
- **Blocked Bots Inspection Menu**: Paginated view of blocked attempts with reason, request path, source IP, and User-Agent.

---

## 🚀 Quick Start with Docker Compose

### 1. Launch with Docker Compose

**Option 1: Build locally from source**
```bash
docker compose up -d --build
```

**Option 2: Use pre-built Docker image from GitHub Container Registry (Instant start, no compiling)**
Edit `docker-compose.yml` to use your published image:
```yaml
services:
  app:
    image: ghcr.io/<your-github-username>/connection-check:latest
```
Then start instantly:
```bash
docker compose up -d
```

The application runs directly on **port 3000**:
- **Public Speedtest UI**: `http://localhost:3000` (or `http://YOUR_SERVER_IP:3000`)
- **Admin Control Panel**: `http://localhost:3000/data`
- **Default Credentials**: `admin` / `admin123` *(prompts to change password on first login)*

---

## 🌐 Reverse Proxy Setup (Nginx & Apache)

When deploying to production with your domain (e.g., `speed.yourdomain.com`), point your DNS `A` record to your server and configure your preferred reverse proxy below.

### Option A: Nginx Configuration

Create `/etc/nginx/sites-available/speedtest.conf`:

```nginx
server {
    listen 80;
    server_name speed.yourdomain.com;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;

        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Real IP forwarding for rate limit and ISP detection
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Disable buffering for real-time speed measurement
        proxy_buffering off;
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }
}
```

Enable the site and obtain a free Let's Encrypt SSL certificate:
```bash
sudo ln -s /etc/nginx/sites-available/speedtest.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d speed.yourdomain.com
```

---

### Option B: Apache Configuration

Ensure the proxy and WebSocket modules are enabled:
```bash
sudo a2enmod proxy proxy_http proxy_wstunnel rewrite ssl headers
```

Create `/etc/apache2/sites-available/speedtest.conf`:

```apache
<VirtualHost *:80>
    ServerName speed.yourdomain.com

    # WebSocket forwarding for real-time latency checks
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://127.0.0.1:3000/$1" [P,L]

    # Main HTTP reverse proxy
    ProxyPreserveHost On
    ProxyPass / http://127.0.0.1:3000/
    ProxyPassReverse / http://127.0.0.1:3000/

    # Real client IP forwarding
    RequestHeader set X-Real-IP "%{REMOTE_ADDR}s"
    RequestHeader set X-Forwarded-Proto "http"
</VirtualHost>
```

Enable the site and obtain SSL with Certbot:
```bash
sudo a2ensite speedtest.conf
sudo systemctl reload apache2
sudo certbot --apache -d speed.yourdomain.com
```

---

## 🔑 Default Credentials

| Field | Value |
|---|---|
| **Admin Route** | `/data` |
| **Username** | `admin` |
| **Default Password** | `admin123` |
| **First Login** | Enforces instant password change before granting access |

---

## ⚙️ Environment Variables

Configurable in `docker-compose.yml`:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3000` | Port the Go application listens on |
| `DB_PATH` | `/app/data/speedtest.db` | Path to persistent SQLite database file |
| `TEMPLATES_DIR` | `/app/web/templates` | Directory containing HTML templates |
| `STATIC_DIR` | `/app/web/static` | Directory containing static CSS and JS |

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
