# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    fonts-liberation \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/server /bin/server

EXPOSE 8080

ENTRYPOINT ["/bin/server"]
