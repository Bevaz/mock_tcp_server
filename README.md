# MOCK TCP CLIENT/SERVER

A TCP server mock, if receive some matched bytes, it will response the specific file data.  
And TCP client mock, send some data and wait for a specific response.  

## Install
```bash
go get github.com/bevaz/mock_tcp_server
```

## Config it

`server.conf`

```json
{
    "mode": "server",
    "host": "0.0.0.0",
    "port": 8080,
    "dump_request": true,
    "requests": [
    {
        "request_type":  "string",
        "request_data":  "REQUEST DATA",
        "response_type": "string",
        "response_data": "RESPONSE DATA"
    }, {
        "request_type":  "byte",
        "request_data":  "616263",
        "response_type": "byte",
        "response_data": "A5A5A5",
        "bye_packet":    true
    }]
}
```

`client.conf`

```json
{
    "mode": "client",
    "host": "127.0.0.1",
    "port": 8080,
    "dump_request": true,
    "requests": [
    {
        "request_type":  "string",
        "request_data":  "REQUEST DATA",
        "response_type": "string",
        "response_data": "RESPONSE DATA"
    }, {
        "request_type":  "byte",
        "request_data":  "616263",
        "response_type": "byte",
        "response_data": "A5A5A5",
        "bye_packet":    true
    }]
}
```

- type:  
  - string  
  - byte  
- data
  - string: just input match string
  - byte:ascii
- dump_request
  - if configured the dump_request = true, will dump the request data to file `./dump/{timestamp}/{ID}.dat`
- bye_packet
  - stop server after receiving this packet

## Start server in console mode

```bash
mock_tcp_server -c server.conf
```

or with docker

```bash
docker run -i -p 8080:8080/tcp -v ${PWD}:/workdir bevaz/mock_tcp_server -c server.conf
```

## Start server in http mode

```bash
mock_tcp_server -h
```

And configure it

```bash
# Reset old config (skip this on start)
curl -X POST http://localhost:8008/mock/reset
# Setup mock rules
curl -X POST http://localhost:8008/mock/setup -d '
{
    "mode": "server",
    "host": "0.0.0.0",
    "port": 8080,
    "dump_request": true,
    "requests": [
    {
        "request_type":  "string",
        "request_data":  "REQUEST DATA",
        "response_type": "string",
        "response_data": "RESPONSE DATA"
    }, {
        "request_type":  "byte",
        "request_data":  "616263",
        "response_type": "byte",
        "response_data": "A5A5A5"
    }]
}
'

```

## Test connection

```bash
mock_tcp_server -c client.conf
```

or

```bash
mock_tcp_server -h &
# Setup mock rules
curl -X POST http://localhost:8008/mock/setup -d '
{
    "mode": "client",
    "host": "0.0.0.0",
    "port": 8080,
    "dump_request": true,
    "requests": [
    {
        "request_type":  "string",
        "request_data":  "REQUEST DATA",
        "response_type": "string",
        "response_data": "RESPONSE DATA"
    }, {
        "request_type":  "byte",
        "request_data":  "616263",
        "response_type": "byte",
        "response_data": "A5A5A5"
    }]
}
'
```
