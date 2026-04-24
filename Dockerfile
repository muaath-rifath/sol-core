FROM golang:1.23-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /sol ./cmd/sol

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /sol /usr/local/bin/sol
COPY migrations /migrations

EXPOSE 8080

ENTRYPOINT ["sol"]
