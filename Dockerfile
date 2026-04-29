# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum* ./
RUN go mod download

# Copy all source files with the new directory structure.
COPY cmd/   ./cmd/
COPY internal/ ./internal/

# Static, stripped, reproducible binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /out/gh-proxy ./cmd/gh-proxy/

# ---- runtime stage ----
# Distroless: no shell, no package manager, runs as nonroot by default.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/gh-proxy /gh-proxy

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gh-proxy"]
