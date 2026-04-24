FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY sol-core-linux-amd64 /usr/local/bin/sol
RUN chmod +x /usr/local/bin/sol
COPY migrations /migrations

EXPOSE 8080

ENTRYPOINT ["sol"]
