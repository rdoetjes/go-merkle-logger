# Merkle Logging Service

This project provides a tamper-evident logging service implemented in Go. It demonstrates a secure-by-design logging approach with hash chaining, HMAC signing, TLS transport (gRPC), and tools to verify log integrity.

Key features
- Structured JSON log lines with chaining and HMAC: sequence, timestamp, previous_hash, payload, current_hash, signature
- gRPC/TLS ingestion (recommended: TLS 1.2+)
- File or syslog backend (configurable)
- Startup rotation of existing log files: existing logfile is renamed to <basename>-<isodatetimestamp><ext>
- CLI utilities: `merkle-client` (example), `merkle-checker`, and `merkle-bench` (throughput runner)
- Integration and unit tests (see Makefile)

Quick build

From the project root (where `go.mod` is located):

```merkle/README.md#L1-6
# ensure modules are downloaded
go mod download
# build server
go build -o merkle-server ./cmd/server
# build client example
go build -o merkle-client ./cmd/client
# build checker tool
go build -o merkle-checker ./cmd/checker
# build bench runner
go build -o merkle-bench ./cmd/bench
```

Generating a test TLS certificate (self-signed, with SANs)

Modern TLS verification requires Subject Alternative Names (SANs). Create an `openssl.conf` with SANs and generate a cert:

```merkle/README.md#L7-16
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout key.pem -out cert.pem \
  -config openssl.cnf -extensions v3_req
```

Or use `-addext` on newer OpenSSL:

```merkle/README.md#L17-18
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

Running the server

Set an HMAC key (recommended to use a KMS/secret store in production):

```merkle/README.md#L19-21
export MERKLE_HMAC_KEY="your-hmac-key-here"
./merkle-server -tls-cert=cert.pem -tls-key=key.pem -addr=:8443 -backend=file -logfile=./protected.log
```

- Startup rotation: if `./protected.log` exists when the server starts, it will be renamed to `protected-<ISO_TIMESTAMP>.log` and a fresh `protected.log` will be created.

Client example

- To run the example client (verifies the server cert with `cert.pem`):

```merkle/README.md#L22-24
./merkle-client -addr=localhost:8443 -ca cert.pem
```

- Quick insecure test (skips TLS verification). Only for local testing:

```merkle/README.md#L25-25
./merkle-client -addr=localhost:8443
```

Checking logs with merkle-checker

`merkle-checker` accepts either `-file` or a positional filename argument. It verifies hash chaining and HMAC signatures (if you provide the key).

Examples:

```merkle/README.md#L26-29
# positional argument
./merkle-checker ./bench.log
# or using the flag
./merkle-checker -file=./bench.log -hmac-key="your-hmac-key"
```

The checker returns exit code `0` on success, non-zero on failure.

Benchmarking / throughput runner

`merkle-bench` is a small CLI that starts an in-process server and client workers to measure write throughput and verify output with the checker. It prints a single `RESULT` line with totals and the logfile path.

Example:

```merkle/README.md#L30-33
# 10 second run, 8 workers, write file to ./bench.log
./bench -duration 10 -workers 8 -out ./bench.log
```

Integration tests and preserving logs

- Unit tests and integration test are available under `internal/`.
- The integration test (`TestIntegrationRate`) runs for 60s by default. You can run it directly with verbose output to see the logfile path and RESULT:

```merkle/README.md#L34-36
# run integration test (verbose) and show logfile path
go test -v -run TestIntegrationRate ./internal/server
```

- To preserve the generated logfile into the project root for inspection, set `MERKLE_PRESERVE_LOG=1` when running the test:

```merkle/README.md#L37-38
MERKLE_PRESERVE_LOG=1 go test -v -run TestIntegrationRate ./internal/server
# Look for PRESERVED_LOG:<path> in the test output
```

Makefile targets

- `make build` — build server, client, checker
- `make test` — run `go test ./...`
- `make integration` — run the integration rate test (verbose)
- `make bench` — build and run `merkle-bench` (default 30s)
- `make cert` — helper to generate self-signed cert (uses `openssl.conf` if present)

Notes & recommendations

- Production: replace the demo HMAC key with a key from KMS/HSM and consider asymmetric signatures for audit non-repudiation.
- Performance: the service fsyncs after each write for durability. For higher throughput, consider batching or background flush with configurable durability tradeoffs.
- Proto code: this demo includes hand-written stub proto types and a JSON codec for simplicity. For production, generate proper Go protobufs with `protoc` + plugins.


## CI

This repository includes a GitHub Actions workflow at `.github/workflows/ci.yml` that runs `go test ./...` on push and pull requests to `main` and uploads a coverage artifact.

![CI](https://github.com/OWNER/REPO/actions/workflows/ci.yml/badge.svg)

License: MIT
