FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy pkg and current service only
COPY pkg/ ./pkg/
COPY services/truck/ ./services/truck/

# Build the service
WORKDIR /app/services/truck

# Add a temporary module replacement that points to the correct path
RUN go mod edit -replace=example.com/backstage/pkg=/app/pkg && \
    go mod tidy && \
    go build -o service .

FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/services/truck/service .

# Expose service port
EXPOSE 8096

CMD ["./service"]
