FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /docgest ./cmd/server

FROM debian:bookworm-slim

# Install poppler-utils for pdftotext fallback.
RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /docgest /docgest

EXPOSE 8090

ENTRYPOINT ["/docgest"]
