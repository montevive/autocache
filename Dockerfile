# Build stage
FROM golang:1.23-alpine AS builder

# Set build arguments for version info
ARG VERSION=1.0.0
ARG BUILD_TIME
ARG GIT_COMMIT

# Install git for go modules
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o autocache ./cmd/autocache

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests to Anthropic API
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S autocache && \
    adduser -u 1001 -S autocache -G autocache

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/autocache .

# Change ownership to non-root user
RUN chown autocache:autocache /app/autocache

# Switch to non-root user
USER autocache

# Expose port (can be overridden with environment variable)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["./autocache"]

# Labels for better container management
LABEL maintainer="autocache@example.com"
LABEL description="Intelligent Anthropic API Cache Proxy"
LABEL version="${VERSION}"