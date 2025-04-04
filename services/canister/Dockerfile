# Build stage
FROM golang:1.20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o canister-service ./cmd/main.go

# Runtime stage
FROM alpine:3.17

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/canister-service .
COPY --from=builder /app/app.env .

# Create a non-root user and set permissions
RUN adduser -D -g '' appuser && \
    chown -R appuser:appuser /app
USER appuser

# Expose the application port
EXPOSE 8080

# Set entry point
ENTRYPOINT ["./canister-service"]

# Default command
CMD ["server"]