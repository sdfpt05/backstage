FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/operations-service

# Final stage
FROM alpine:3.17

WORKDIR /app

# Install CA certificates
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/operations-service /app/operations-service
COPY --from=builder /app/config /app/config

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Expose port
EXPOSE 8000

# Set entry point
ENTRYPOINT ["/app/operations-service"]
CMD ["serve"]