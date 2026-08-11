# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o worker-service ./cmd/...

# ── Stage 2: Run ──────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/worker-service .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8084

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:8084/healthz || exit 1

CMD ["./worker-service"]