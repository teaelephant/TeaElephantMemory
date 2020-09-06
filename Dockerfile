FROM golang:1.14-alpine AS build

WORKDIR /build/

RUN mkdir /build/etc/
RUN mkdir /build/app/

ADD . .

RUN apk add git ca-certificates && update-ca-certificates
RUN adduser -D -g '' appuser

RUN go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o ./app/ ./...

FROM scratch

COPY --from=0 build/app /app
#COPY --from=0 etc/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 etc/passwd /etc/passwd

WORKDIR /app/
USER appuser
ENTRYPOINT ["/app/server"]