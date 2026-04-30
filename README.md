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

1. Download dependencies and build

Run:

- Install dependencies

  go mod download

- Build the binaries

  make build

(You can also run `go build` directly: `go build -o merkle-server ./cmd/server` etc.)

Protocol buffers (proto) usage

This repository uses a protobuf contract at `proto/logging.proto` as the canonical definition for the gRPC API between clients and the server. The generated Go files live under `proto/` and are used by the server and client.

To generate Go code from the .proto files locally, the project provides helper targets in the Makefile.

- Install the code generation tools and generate Go files:

  make proto

This target will:
- install `protoc-gen-go` and `protoc-gen-go-grpc` via `go install` (developer convenience)
- run `protoc --go_out=. --go-grpc_out=. proto/*.proto` to produce `proto/*.pb.go` files

Requirements for `make proto`:
- `protoc` (the Protocol Buffers compiler) must be installed and available on PATH. On macOS you can install with Homebrew:

  brew install protobuf

- `make proto` will install the Go plugins (`protoc-gen-go` and `protoc-gen-go-grpc`) into your `GOBIN` or `GOPATH/bin`.

Why we generate protos
- Generated code gives you strongly typed message structs and efficient protobuf binary encoding. This is the correct approach for production gRPC services.

TLS certificate

Create a test self-signed certificate with SANs for `localhost`/`127.0.0.1` as described previously (see `openssl.conf` in the repo) or use the `gen_certificate.sh` helper script if present.

Run the server

Set an HMAC key (in production, use a secrets manager/KMS):

  export MERKLE_HMAC_KEY="your-hmac-key-here"
  ./merkle-server -tls-cert=cert.pem -tls-key=key.pem -addr=:8443 -backend=file -logfile=./protected.log

- Startup rotation: if `./protected.log` exists when the server starts, it will be renamed to `protected-<ISO_TIMESTAMP>.log` and a fresh `protected.log` will be created.

Run the client (example)

- Using CA verification:

  ./merkle-client -addr=localhost:8443 -ca cert.pem

- Quick insecure test (local only):

  ./merkle-client -addr=localhost:8443

Checker and bench tools

- `merkle-checker` verifies hash chaining and signatures. Usage:

  ./merkle-checker ./bench.log
  ./merkle-checker -file=./bench.log -hmac-key="your-hmac-key"

- `merkle-bench` is a local throughput runner that starts an in-process server and clients, writes a log file and runs the checker automatically. Example:

  ./merkle-bench -duration 10 -workers 8 -out ./bench.log

Testing and integration

- Run unit tests:

  make test

- Integration & bench tests are intentionally disabled by default when running `go test ./...`. To run the integration benchmark test (long running) set:

  MERKLE_RUN_INTEGRATION=1 go test -v -run TestIntegrationRate ./internal/server

- To preserve the integration-generated logfile into the project root during the test run, set:

  MERKLE_PRESERVE_LOG=1 MERKLE_RUN_INTEGRATION=1 go test -v -run TestIntegrationRate ./internal/server

CI (GitHub Actions)

A workflow is included at `.github/workflows/ci.yml` that:
- installs Go
- downloads modules
- runs `protoc --go_out` and `protoc --go-grpc_out` to generate proto code
- runs `go test -v ./...`
- uploads a coverage artifact

A placeholder badge is included in this README — update it after you push the repo to GitHub and replace `OWNER/REPO` with your repository details.

Notes & recommendations

- Production: replace the demo HMAC key with a key from KMS/HSM and consider asymmetric signatures for audit non-repudiation.
- Performance: the service fsyncs after each write for durability. For higher throughput, consider batching or background flush with configurable durability tradeoffs.
- Proto code: generated protobuf & gRPC Go files are used in this repo (run `make proto` to regenerate). Keep the .proto file as the single source of truth.

License: MIT
