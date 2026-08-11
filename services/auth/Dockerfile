# ── Stage 1: Build ────────────────────────────────────────────
# Use official Go image to compile the binary
FROM golang:1.26-alpine AS builder

# Install git (needed for go modules)
RUN apk add --no-cache git

# Set working directory inside container
WORKDIR /app

# Copy go.mod and go.sum first
# Docker caches this layer — only re-downloads deps if these files change
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy all source code
COPY . .

# Build the binary
# CGO_ENABLED=0 — pure Go binary, no C dependencies
# GOOS=linux — build for Linux (EKS runs Linux)
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service ./cmd/...

# ── Stage 2: Run ──────────────────────────────────────────────
# Use minimal alpine image — only 5MB
# Much smaller than the builder image (300MB+)
FROM alpine:3.19

# Add CA certificates for HTTPS calls
RUN apk add --no-cache ca-certificates wget

# Create non-root user for security
# Never run containers as root in production
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy only the compiled binary from builder stage
COPY --from=builder /app/auth-service .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8081

# Health check — Docker and Kubernetes use this
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:8081/healthz || exit 1

# Start the service
CMD ["./auth-service"]