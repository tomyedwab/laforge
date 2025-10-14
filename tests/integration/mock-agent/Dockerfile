FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY mock_agent.go .
RUN go build -o mock-agent mock_agent.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /workspace
COPY --from=builder /app/mock-agent /usr/local/bin/mock-agent
ENTRYPOINT ["/usr/local/bin/mock-agent"]