FROM golang:1.24-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY tasks ./tasks
RUN go build -o /bin/latasks ./cmd/latasks

FROM debian:bookworm-slim

# Install packages and clean up temporary files
RUN apt update && apt install -y curl zip sqlite3 && apt clean && rm -rf /var/lib/apt/lists/*

# Copy the latasks binary from builder
COPY --from=builder /bin/latasks /bin/latasks

# Create a non-root user and group
RUN adduser --disabled-password --gecos "" laforge
RUN mkdir -p /home/laforge/.config/opencode /home/laforge/.local/share/opencode /src /state
RUN chown -R laforge:laforge /home/laforge /state

# Switch to the non-root user
USER laforge
WORKDIR /home/laforge

# Install opencode
RUN curl -fsSL https://opencode.ai/install | bash

WORKDIR /src

#ENTRYPOINT ["/home/opencode/.opencode/bin/opencode"]
