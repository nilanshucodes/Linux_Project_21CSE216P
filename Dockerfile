# Build the Go application against SQLite's system library.
FROM golang:1.27-alpine3.24 AS builder

WORKDIR /src
RUN apk add --no-cache build-base sqlite-dev
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/club ./cmd/club

FROM alpine:3.24
RUN apk add --no-cache sqlite-libs libgcc \
    && addgroup -S club && adduser -S -G club club \
    && mkdir -p /data && chown -R club:club /data
WORKDIR /app
COPY --from=builder /out/club /usr/local/bin/club
ENV CLUB_DB_PATH=/data/club.db
USER club
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/club"]
