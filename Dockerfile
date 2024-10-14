# Use Debian 10 (Buster) as the base image
FROM debian:buster

# Install build dependencies (libpam, git, curl) and the correct version of glibc
RUN apt-get update && \
    apt-get install -y git curl build-essential wget && \
    apt-get install -y libpam0g=1.3.1-5 libpam0g-dev=1.3.1-5 && \
    rm -rf /var/lib/apt/lists/*

# Download and install Go 1.22.2
RUN curl -LO https://go.dev/dl/go1.22.2.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz && \
    rm go1.22.2.linux-amd64.tar.gz

# Set Go paths
ENV PATH="/usr/local/go/bin:${PATH}"

# Set the working directory
WORKDIR /app

# Copy project files
COPY . .

# Build the application
RUN go mod tidy && go build -o /app/bin/file_server .

# Set the default command
CMD ["/app/bin/file_server"]
