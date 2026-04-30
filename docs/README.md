# Merkle Logging Service

This project provides a small tamper-evident logging service implemented in Go.

Goals
- Provide structured, chained logs suitable for tamper-evident audit trails
- Accept logs over TLS (gRPC)
- Support file and syslog backends
- Include a CLI tool to check log consistency


Quick build

From the project root (where `go.mod` is located):

1. Download dependencies and build

```sh
# ensure modules are downloaded
go mod download
# build server
go build -o merkle-server ./cmd/server
# build client example
go build -o merkle-client ./cmd/client
# build checker tool
go build -o merkle-checker ./cmd/checker
```

Creating a test TLS certificate (self-signed, with SANs)

Modern TLS verification requires Subject Alternative Names (SANs). Use this openssl config example to create a cert valid for `localhost` and `127.0.0.1`.

Create `openssl.cnf` with the following content:

```ini
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = localhost

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
```

Then generate a key and self-signed cert:

```sh
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout key.pem -out cert.pem \
  -config openssl.cnf -extensions v3_req
```

Alternatively on systems with OpenSSL that supports `-addext`:

```sh
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

Run the server

```sh
export MERKLE_HMAC_KEY="your-hmac-key-here"
./merkle-server -tls-cert=cert.pem -tls-key=key.pem -addr=:8443 -backend=file -logfile=./protected.log
```

Run the client (example)

```sh
# using CA verification against the self-signed cert
./merkle-client -addr=localhost:8443 -ca cert.pem

# or quick local test (insecure; skips verification)
./merkle-client -addr=localhost:8443
```

Check the protected log

```sh
# run checker tool
./merkle-checker -file=./protected.log -hmac-key="your-hmac-key-here"
```

Notes & next steps

- This repository includes small hand-written proto stubs and a JSON codec for demo purposes. For production, generate real Go protobuf code with `protoc` and the `protoc-gen-go` and `protoc-gen-go-grpc` plugins.
- Use a KMS or secrets manager to store signing keys. Consider switching to asymmetric signatures for stronger non-repudiation.
- For high-volume logging, consider batching writes or a background worker to reduce fsync overhead.

License: MIT
