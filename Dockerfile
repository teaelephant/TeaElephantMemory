FROM golang:1.25

WORKDIR /tmp

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

ARG GOPROXY
ENV \
  GO111MODULE=on \
  CGO_ENABLED=1 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /go/src/github.com/lueurxax/teaelephantmemory/
ADD go.mod go.sum /go/src/github.com/lueurxax/teaelephantmemory/
RUN go mod download -x

ADD . .

ARG VERSION
RUN go build -v -ldflags="-w -s -X main.version=${VERSION}" -o /bin/server cmd/server/*.go

CMD /bin/server