# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install git (required by Gitea SDK during go mod download)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orchestrator ./cmd/serve

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/orchestrator .

# Expose port 8080
EXPOSE 8080

# Run the binary
CMD ["./orchestrator"]
