# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o okaproxy .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl tzdata && \
    addgroup -g 1001 -S okaproxy && \
    adduser -u 1001 -S okaproxy -G okaproxy

# Set timezone
ENV TZ=UTC

# Create necessary directories
RUN mkdir -p /app/logs /app/public && \
    chown -R okaproxy:okaproxy /app

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/okaproxy .
COPY --chown=okaproxy:okaproxy public/ ./public/
COPY --chown=okaproxy:okaproxy config.toml.example .

# Make binary executable
RUN chmod +x okaproxy

# Switch to non-root user
USER okaproxy

# Expose ports (adjust based on your configuration)
EXPOSE 3000 3001 3002 3443

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3000/health || exit 1

# Run the application
CMD ["./okaproxy", "--config", "config.toml"]

# Metadata
LABEL maintainer="OkaProxy Team"
LABEL version="1.0.0"
LABEL description="High-performance HTTP proxy with DDoS protection"
LABEL org.opencontainers.image.source="https://github.com/GentsunCheng/okaproxy"
LABEL org.opencontainers.image.documentation="https://github.com/GentsunCheng/okaproxy/blob/main/README.md"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Build arguments for metadata
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

LABEL org.opencontainers.image.created=$BUILD_DATE
LABEL org.opencontainers.image.revision=$VCS_REF
LABEL org.opencontainers.image.version=$VERSION