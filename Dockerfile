# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1: Build binary
# ==============================================================================
FROM golang:1.24-alpine AS builder

# Install build tools and certificates
RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /build

# Leverage Docker cache for dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy application source
COPY . .

# Build statically linked binary with CGO_ENABLED=0 (using modernc.org/sqlite pure Go driver)
# -trimpath strips absolute paths from the binary for reproducible builds and security
# -ldflags="-s -w" strips symbol table and debug info to minimize binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /bin/bot \
    ./cmd/main.go

# ==============================================================================
# Stage 2: Minimal runtime image
# ==============================================================================
FROM alpine:3.21

# Install runtime dependencies:
# - ca-certificates: essential for TLS/HTTPS communication with Telegram Bot API
# - tzdata: required for time.LoadLocation (e.g. Europe/Kyiv) in application logging
RUN apk add --no-cache ca-certificates tzdata

# Create dedicated unprivileged user and group (never run as root in production)
RUN addgroup -S -g 10001 botgroup && \
    adduser -S -u 10001 -G botgroup -h /home/botuser -s /sbin/nologin botuser

WORKDIR /app

# Create data directory for SQLite storage (or volume mount point) and config directory
RUN mkdir -p /app/configs /data && \
    chown -R botuser:botgroup /app /data

# Copy compiled binary and configuration
COPY --from=builder --chown=botuser:botgroup /bin/bot /app/bot
COPY --from=builder --chown=botuser:botgroup /build/configs/config.json /app/configs/config.json

# Switch to unprivileged non-root user
USER botuser:botgroup

# Default environment variables
ENV PORT=8080 \
    WEBHOOK_PATH=/bot \
    DB_PATH=/data/bot.db

# Expose standard Cloud Run HTTP port
EXPOSE 8080

# Health check endpoint for container orchestrators
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:${PORT}/healthz || exit 1

ENTRYPOINT ["/app/bot"]
