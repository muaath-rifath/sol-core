# Stage 1: Build the worker binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o sol-builder ./main.go

# Stage 2: Final image with ESP-IDF
FROM espressif/idf:release-v5.3
WORKDIR /app
COPY --from=builder /app/sol-builder /usr/local/bin/sol-builder
RUN chmod +x /usr/local/bin/sol-builder

# Workspace for builds
WORKDIR /firmware
ENTRYPOINT ["sol-builder"]
