FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 go build -o viewer ./cmd/serve

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

# Copy binary from builder
COPY --from=builder /build/viewer /app/viewer

# Copy templates and static files
COPY templates /app/templates
COPY static /app/static

# Create data directory for SQLite database
RUN mkdir -p /data

# Expose port
EXPOSE 8082

# Run the viewer
CMD ["/app/viewer"]
