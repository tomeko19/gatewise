# Multi-stage build for Gatewise Community Edition
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY gatewise/go.mod gatewise/go.sum ./
RUN go mod download

# Copy source code
COPY gatewise/ ./

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gatewise-server ./cmd/gatewise-server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gatewise-operator ./cmd/gatewise-operator

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binaries from builder
COPY --from=builder /build/gatewise-server /app/
COPY --from=builder /build/gatewise-operator /app/

# Copy configuration examples
COPY gatewise/configs/ /app/configs/

# Create non-root user
RUN addgroup -g 1000 gatewise && \
    adduser -D -u 1000 -G gatewise gatewise && \
    chown -R gatewise:gatewise /app

USER gatewise

# Expose ports
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8001/api/health || exit 1

# Default command
CMD ["/app/gatewise-server", "--db-url", "${DATABASE_URL}"]
