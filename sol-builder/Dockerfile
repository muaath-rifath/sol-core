FROM espressif/idf:release-v5.3

WORKDIR /app

# Copy the pre-built binary
COPY sol-builder-linux-amd64 /usr/local/bin/sol-builder
RUN chmod +x /usr/local/bin/sol-builder

# Workspace for builds
WORKDIR /firmware

ENTRYPOINT ["sol-builder"]
