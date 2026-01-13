# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o audioconv ./cmd/audioconv

# Runtime stage
FROM alpine:3.19

WORKDIR /data

# Copy binary from builder
COPY --from=builder /app/audioconv /usr/local/bin/audioconv

# Default command
ENTRYPOINT ["audioconv"]
CMD ["--help"]

LABEL org.opencontainers.image.title="audioconv"
LABEL org.opencontainers.image.description="Pure Go audio converter without ffmpeg"
LABEL org.opencontainers.image.source="https://github.com/formeo/go-audio-converter"
