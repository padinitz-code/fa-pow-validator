FROM golang:alpine as build

RUN apk add ca-certificates git gcc musl-dev mingw-w64-gcc

WORKDIR /opt

COPY go.mod go.sum ./
RUN  go mod download

COPY cmd/server      cmd/server
COPY internal/pow    internal/pow
COPY internal/server internal/server

RUN go test -cover -race -v ./...

RUN cd /opt/cmd/server && \
    go build -o /srv/server


FROM alpine:latest

RUN apk add --no-cache nginx

COPY --from=build /srv /srv
COPY deploy/nginx.conf /etc/nginx/nginx.conf

WORKDIR /srv

EXPOSE 9000 8080
EXPOSE 8443

# Remove old healthcheck and add new one for /healthz
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --spider -q http://localhost/healthz || exit 1

CMD ["/bin/sh", "-c", "nginx && /srv/server"]
