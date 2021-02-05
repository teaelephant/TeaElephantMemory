FROM golang:1.15 AS build

WORKDIR /build/

RUN mkdir /build/etc/
RUN mkdir /build/app/

ADD . .

RUN \
apt-get update -q && \
apt-get install -yq pkg-config m4 default-jdk mono-devel git gcc ca-certificates libc6-dev --no-install-recommends && \
apt-get autoclean -yq && \
apt-get clean -yq

RUN wget https://www.foundationdb.org/downloads/6.2.28/ubuntu/installers/foundationdb-clients_6.2.28-1_amd64.deb
RUN dpkg -i foundationdb-clients_6.2.28-1_amd64.deb
RUN chmod +x ./fdb-go-install.sh

RUN ./fdb-go-install.sh install --fdbver 6.2.28

ENV CGO_CPPFLAGS="-I/go/src/github.com/apple/foundationdb/bindings/c" CGO_CFLAGS="-g -O2" CGO_LDFLAGS="-L/usr/lib"

#RUN go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o ./app/ ./...
RUN go build -o ./app/ ./...

FROM scratch

COPY --from=0 build/app /app
#COPY --from=0 etc/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 etc/passwd /etc/passwd

WORKDIR /app/
ENTRYPOINT ["/app/server"]