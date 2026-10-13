# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install git and ca-certificates if needed for module downloads
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build pure Go binary (CGO_ENABLED=0)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o connection-check ./cmd/server

# Final Runtime Stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Create data directory for SQLite database persistence
RUN mkdir -p /app/data

# Copy binary and assets
COPY --from=builder /build/connection-check /app/connection-check
COPY --from=builder /build/web /app/web

# Set environment defaults
ENV PORT=3000 \
    DB_PATH=/app/data/speedtest.db \
    TEMPLATES_DIR=/app/web/templates \
    STATIC_DIR=/app/web/static \
    CADDY_ADMIN_URL=http://caddy:2019

EXPOSE 3000

VOLUME ["/app/data"]

CMD ["/app/connection-check"]
