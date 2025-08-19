FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimization and static linking
RUN CGO_ENABLED=1 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags '-extldflags "-static" -s -w' \
    -o shelly-manager ./cmd/shelly-manager

# Final stage
FROM alpine:latest

# Security hardening
RUN apk --no-cache add ca-certificates sqlite tzdata \
    && adduser -D -u 10001 -s /bin/sh shellyuser \
    && rm -rf /var/cache/apk/*

# Set working directory
WORKDIR /app

# Create data directory with proper ownership
RUN mkdir -p /data \
    && chown -R shellyuser:shellyuser /data /app

# Copy binary from builder
COPY --from=builder /app/shelly-manager ./shelly-manager

# Copy web files and config with proper ownership
COPY --chown=shellyuser:shellyuser web/ ./web/
COPY --chown=shellyuser:shellyuser configs/ ./configs/

# Switch to non-root user
USER shellyuser

# Expose port
EXPOSE 8080

# Create volume for data persistence
VOLUME ["/data"]

# Set environment variables
ENV SHELLY_DATABASE_PATH=/data/shelly.db \
    GIN_MODE=release

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["./shelly-manager", "--version"] || exit 1

# Graceful shutdown signal
STOPSIGNAL SIGTERM

CMD ["./shelly-manager", "server"]
