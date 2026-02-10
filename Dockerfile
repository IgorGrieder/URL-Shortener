# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api_hightps
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/outbox-worker ./cmd/outbox_worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/click-consumer ./cmd/click_consumer

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /app/bin/api .
COPY --from=builder /app/bin/outbox-worker .
COPY --from=builder /app/bin/click-consumer .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./api"]
