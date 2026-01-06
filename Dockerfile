# Use Ubuntu 24.04 as the build stage base image
FROM ubuntu:24.04 AS builder

# Set environment variables for non-interactive installs
ENV DEBIAN_FRONTEND=noninteractive
#ENV GO_VERSION=1.23.2

# Install required packages
RUN apt-get update -q && \
    apt-get install --no-install-recommends -y -q \
        wget \
        curl \
        git \
        build-essential \
        ca-certificates \
        tzdata \
        locales && \
    apt-get autoremove -y -q && \
    apt-get clean -y -q && \
    rm -rf /var/lib/apt/lists/* /usr/share/doc /usr/share/man /var/cache/* && \
    localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8

# Download Go
RUN GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -n 1) && \
    wget -nv https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf ${GO_VERSION}.linux-amd64.tar.gz && \
    rm ${GO_VERSION}.linux-amd64.tar.gz

# Set environment variables
ENV LANG en_US.utf8
ENV TZ=Asia/Seoul

# Configure timezone
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

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
RUN apt-get update -q && \
    apt-get install --no-install-recommends -y -q \
        ca-certificates \
        tzdata \
        locales && \
    apt-get autoremove -y -q && \
    apt-get clean -y -q && \
    rm -rf /var/lib/apt/lists/* && \
    localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8

# Set environment variables
ENV LANG en_US.utf8
ENV TZ=Asia/Seoul

# Configure timezone
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

# Disable/remove package manager and GPG tooling in runtime image
# Note: Builder stage intentionally keeps apt-get; runtime stage does not.
RUN apt-get update -q || true; \
    apt-get purge -y -q --auto-remove \
        openssh-client openssh-server \
        netcat-openbsd \
        net-tools \
        iputils-ping \
        telnet \
        traceroute \
        curl \
        wget \
        apt-utils || true; \
    dpkg --purge --force-all gpgv gnupg gnupg-utils || true; \
    dpkg --purge --force-all apt apt-utils || true; \
    rm -f /usr/bin/apt /usr/bin/apt-get /usr/bin/apt-cache /usr/bin/apt-config \
          /usr/bin/gpgv /usr/bin/gpg /usr/bin/gpgconf /usr/bin/gpg-agent || true; \
    rm -rf /etc/apt /var/lib/apt /var/cache/apt /var/lib/apt/lists/* /root/.gnupg || true; \
    rm -f /bin/apt /bin/apt-get /usr/local/bin/apt /usr/local/bin/apt-get || true

# Copy binary from builder stage
COPY --from=builder /app/bin/owntracks2ha /app/bin/owntracks2ha
COPY --from=builder /app/config /app/config

# Ensure the binary path is in the environment PATH
ENV PATH="/app/bin:$PATH"

# Set working directory
WORKDIR /app

# Set the default command to run the application
CMD ["owntracks2ha"]
