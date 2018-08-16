FROM alpine

ENV GOROOT=/usr/lib/go \
    GOPATH=/go \
    GOBIN=/go/bin \
    PATH=$PATH:$GOROOT/bin:$GOPATH/bin

WORKDIR /go/src/mock_tcp_server
COPY . .

RUN apk add --no-cache git go musl-dev && \
  go get -d -v ./... && \
  go install -v ./... && \
  apk del git go musl-dev && \
  rm -rf /go/pkg && \
  rm -rf /go/src

VOLUME [ "/workdir" ]
WORKDIR /workdir

ENTRYPOINT ["/go/bin/mock_tcp_server"]
