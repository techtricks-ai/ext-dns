# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Copy source code
COPY . .

# Fetch dependencies since we couldn't run go mod tidy locally
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o docker-external-dns ./cmd/docker-external-dns

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for TLS (Cloudflare API)
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/docker-external-dns .

CMD ["./docker-external-dns"]
