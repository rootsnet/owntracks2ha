# Use Ubuntu 24.04 as the build stage base image
FROM ubuntu:24.04 AS builder

# Set environment variables for non-interactive installs and Go version
ENV DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.23.2

# Install required packages and download Go
RUN apt update && apt install -y --no-install-recommends \
    wget git curl ca-certificates build-essential && \
    wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

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
    go get github.com/eclipse/paho.mqtt.golang@v1.5.0 && \
    go get github.com/gorilla/websocket@v1.5.3 && \
    go get golang.org/x/net@v0.27.0 && \
    go get golang.org/x/sync@v0.7.0 && \
    go get gopkg.in/yaml.v2@v2.4.0

# Build the application binary
RUN mkdir -p /app/bin && \
    go build -o /app/bin/owntracks2ha /app/src/main.go -v -ldflags "-s -w"

# ───────────────────────────────────────────────

# Use a clean runtime image
FROM ubuntu:24.04

# Install CA certificates for TLS support
RUN apt update && apt install -y --no-install-recommends ca-certificates && \
    apt clean && rm -rf /var/lib/apt/lists/*

# Copy the binary from the builder stage
COPY --from=builder /app/bin/owntracks2ha /usr/local/bin/owntracks2ha

# Ensure the binary path is in the environment PATH
ENV PATH="/usr/local/bin:$PATH"

# Set working directory to root
WORKDIR /

# Set the default command to run the application
CMD ["owntracks2ha"]
