# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sol ./cmd/sol/main.go

# Final stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/sol /usr/local/bin/sol
COPY migrations /migrations

EXPOSE 8080

ENTRYPOINT ["sol"]
