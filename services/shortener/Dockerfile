# Build stage
FROM golang:1.24.4-alpine AS builder
WORKDIR /app

# Install gcc for CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

# Copy modules and download dependencies first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=1 GOOS=linux go build -o shortener-service ./services/shortener

# Runtime stage - optimized for live updates
FROM golang:1.24.4-alpine
WORKDIR /app

# Install gcc for live rebuilds
RUN apk add --no-cache gcc musl-dev

# Copy binary and source (for live updates)
COPY --from=builder /app/shortener-service .
COPY --from=builder /app/go.mod /app/go.sum ./
COPY --from=builder /app/services/shortener ./services/shortener/
COPY --from=builder /app/shared ./shared/

EXPOSE 50051
CMD ["./shortener-service"]