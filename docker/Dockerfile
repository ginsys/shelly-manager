FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o shelly-manager ./cmd/shelly-manager

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/

# Create data directory
RUN mkdir -p /data

# Copy binary from builder
COPY --from=builder /app/shelly-manager .

# Copy web files and config
COPY web/ ./web/
COPY configs/ ./configs/

# Expose port
EXPOSE 8080

# Create volume for data persistence
VOLUME ["/data"]

# Set environment variables
ENV SHELLY_DATABASE_PATH=/data/shelly.db

CMD ["./shelly-manager", "server"]
