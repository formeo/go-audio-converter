# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o audioconv ./cmd/audioconv

# Runtime stage
FROM alpine:3.19

# Add ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/audioconv /usr/local/bin/audioconv

# Create non-root user
RUN adduser -D -u 1000 audioconv
USER audioconv

# Create working directory
WORKDIR /data

# Default command
ENTRYPOINT ["audioconv"]
CMD ["--help"]

# Labels
LABEL org.opencontainers.image.title="audioconv"
LABEL org.opencontainers.image.description="Pure Go audio converter without ffmpeg"
LABEL org.opencontainers.image.source="https://github.com/formeo/go-audio-converter"
