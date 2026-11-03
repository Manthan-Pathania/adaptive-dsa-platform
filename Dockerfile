# ==============================================================================
# Adaptive DSA Platform - Go Modular Monolith Multi-Stage Dockerfile
# ==============================================================================

# Stage 1: Builder
FROM golang:1.23-alpine AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates git tzdata

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code and migrations
COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/

# Build statically linked binaries with stripped symbols for minimal size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/migrate ./cmd/migrate

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/admin ./cmd/admin

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/import-questions ./cmd/import-questions

# Stage 2: Minimal hardened runtime
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrate /app/migrate
COPY --from=builder /app/admin /app/admin
COPY --from=builder /app/import-questions /app/import-questions
COPY --from=builder /build/migrations /app/migrations

# Non-root runtime user
USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/server"]
