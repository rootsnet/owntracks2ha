# Use Ubuntu 24.04 as the build stage base image
FROM ubuntu:24.04 AS builder

# Set environment variables for non-interactive installs and Go version
ENV DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.23.2

# Install required packages and download Go
RUN apt update && apt install -y --no-install-recommends \
    wget curl git ca-certificates build-essential tzdata && \
    GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -n 1) && \
    wget -nv https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf ${GO_VERSION}.linux-amd64.tar.gz && \
    rm ${GO_VERSION}.linux-amd64.tar.gz && \
    ln -sf /usr/share/zoneinfo/Asia/Seoul /etc/localtime && \
    echo "Asia/Seoul" > /etc/timezone

# Set Go environment variables
ENV GOROOT=/usr/local/go \
    GOPATH=/app \
    PATH=$PATH:/usr/local/go/bin:/app/bin

# Create working directory for source
WORKDIR /app

# Clone the project repository
RUN git clone https://github.com/rootsnet/owntracks2ha.git .

# Initialize Go module and install dependencies
WORKDIR /app/src
RUN go mod init owntracks2ha && \
    go get github.com/eclipse/paho.mqtt.golang && \
    go get github.com/gorilla/websocket && \
    go get golang.org/x/net && \
    go get golang.org/x/sync && \
    go get gopkg.in/yaml.v2

# Build the application binary
RUN mkdir -p /app/bin && \
    go build -o /app/bin/owntracks2ha /app/src/main.go

# ───────────────────────────────────────────────

# Use a clean runtime image
FROM ubuntu:24.04

# Install CA certificates for TLS support
RUN apt update && apt install -y --no-install-recommends \
    ca-certificates tzdata && \
    ln -sf /usr/share/zoneinfo/Asia/Seoul /etc/localtime && \
    echo "Asia/Seoul" > /etc/timezone && \
    apt clean && rm -rf /var/lib/apt/lists/*

# Copy binary from builder stage
COPY --from=builder /app/bin/owntracks2ha /app/bin/owntracks2ha
COPY --from=builder /app/config /app/config

# Ensure the binary path is in the environment PATH
ENV PATH="/app/bin:$PATH"

# Set working directory to root
WORKDIR /app

# Set the default command to run the application
CMD ["owntracks2ha"]
