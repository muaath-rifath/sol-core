FROM espressif/idf:release-v5.3

# Install Go to build the worker
RUN apt-get update && apt-get install -y wget && \
    wget https://go.dev/dl/go1.23.2.linux-amd64.tar.gz && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz && \
    rm go1.23.2.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

# Copy the worker source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the worker binary
RUN go build -o /usr/local/bin/sol-builder ./main.go

# Workspace for builds
WORKDIR /firmware

# The source code will be mounted at /firmware or cloned here
# For now we assume it's mounted via volume in docker-compose

ENTRYPOINT ["sol-builder"]
