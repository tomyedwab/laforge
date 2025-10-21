FROM golang:1.24-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY tasks ./tasks
RUN go build -o /bin/latasks ./cmd/latasks

FROM debian:bookworm-slim

# Install packages and clean up temporary files
RUN apt update && apt install -y curl zip sqlite3 curl && apt clean && rm -rf /var/lib/apt/lists/*

# Create a non-root user and group
RUN adduser --disabled-password --gecos "" laforge
RUN mkdir -p /home/laforge/.config/opencode /home/laforge/.local/share/opencode /src /state
RUN chown -R laforge:laforge /home/laforge /state

# Switch to the non-root user
USER laforge
WORKDIR /home/laforge

# Install Node.js and Claude Code
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash
RUN bash -l -c ". /home/laforge/.nvm/nvm.sh && nvm install 22 && npm install -g @anthropic-ai/claude-code"

# Install opencode
RUN curl -fsSL https://opencode.ai/install | bash

# Copy the latasks binary & scripts from builder
COPY --from=builder /bin/latasks /bin/latasks
COPY scripts/*.sh /bin/
ADD scripts/.claude /bin/.claude

WORKDIR /src
