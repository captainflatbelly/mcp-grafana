# Build stage
FROM golang:1.24-bullseye AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 🔧 Cross-compile for Linux (which is what your container will run)
RUN GOOS=linux GOARCH=amd64 go build -o mcp-grafana-binary ./cmd/mcp-grafana

# Final stage
FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 1000 -m mcp-grafana
WORKDIR /app

# Copy files from builder
COPY --from=builder --chown=1000:1000 /app /app

USER mcp-grafana
EXPOSE 8000
ENTRYPOINT ["./mcp-grafana-binary", "--transport", "sse", "--sse-address", "0.0.0.0:8000"]
