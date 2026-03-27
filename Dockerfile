FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/id-generator-server ./cmd/id-generator-server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget && \
    adduser -D -H -u 10001 appuser && \
    mkdir -p /data /app/logs

WORKDIR /app

COPY --from=builder /out/id-generator-server /app/id-generator-server

USER appuser

EXPOSE 8088 8011

ENTRYPOINT ["/app/id-generator-server"]
