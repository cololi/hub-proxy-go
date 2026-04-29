# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum* ./
RUN go mod download

# Copy all source files
COPY cmd/   ./cmd/
COPY internal/ ./internal/

# Static, stripped, reproducible binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /out/hub-proxy-go ./cmd/hub-proxy-go/

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/hub-proxy-go /hub-proxy-go

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/hub-proxy-go"]
