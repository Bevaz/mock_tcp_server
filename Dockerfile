FROM golang:alpine

WORKDIR /go/src/mock_tcp_server
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

WORKDIR /workdir

ENTRYPOINT ["mock_tcp_server"]
