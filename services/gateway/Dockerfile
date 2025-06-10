# Build stage
FROM golang:1.24.4-alpine AS builder
WORKDIR /app

# Copy modules and download dependencies first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway-service ./services/gateway

# Runtime stage - optimized for live updates
FROM golang:1.24.4-alpine
WORKDIR /app

# Copy binary and source (for live updates)
COPY --from=builder /app/gateway-service .
COPY --from=builder /app/go.mod /app/go.sum ./
COPY --from=builder /app/services/gateway ./services/gateway/
COPY --from=builder /app/shared ./shared/

EXPOSE 8080
CMD ["./gateway-service"]