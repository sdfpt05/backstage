FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy pkg and current service only
COPY pkg/ ./pkg/
COPY services/sales/ ./services/sales/

# Build the service
WORKDIR /app/services/sales

# Add a temporary module replacement that points to the correct path
RUN go mod edit -replace=example.com/backstage/pkg=/app/pkg && \
    go mod tidy && \
    go build -o service .

FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/services/sales/service .

# Expose service port
EXPOSE 8094

CMD ["./service"]
