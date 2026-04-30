# Makefile for merkle logging service

.PHONY: all build server client checker cert run-server run-client clean

all: build

build: proto
	@echo "building binaries (proto files generated)"
	go build -o merkle-server ./cmd/server
	go build -o merkle-client ./cmd/client
	go build -o merkle-checker ./cmd/checker
	go build -o merkle-bench ./cmd/bench

proto: install-proto-tools
	@echo "generating go files from proto/*.proto"
	protoc --go_out=. --go-grpc_out=. proto/*.proto

install-proto-tools:
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc into $(go env GOPATH)/bin or $(go env GOBIN)"
	@echo "This may take a while the first time."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "installed protoc plugins"

server:
	go build -o merkle-server ./cmd/server

client:
	go build -o merkle-client ./cmd/client

checker:
	go build -o merkle-checker ./cmd/checker

cert:
	@echo "Generating self-signed certificate (with SANs) as cert.pem/key.pem"
	@if [ -f openssl.conf ]; then \
		openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		  -keyout key.pem -out cert.pem \
		  -config openssl.conf -extensions v3_req; \
	else \
		openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem \
		  -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"; \
	fi

run-server:
	@echo "Starting server (requires cert.pem and key.pem). Use MERKLE_HMAC_KEY env var to set HMAC key."
	export MERKLE_HMAC_KEY=${MERKLE_HMAC_KEY:-demo-key}; ./merkle-server -tls-cert=cert.pem -tls-key=key.pem -addr=:8443 -backend=file -logfile=./protected.log

run-client:
	./merkle-client -addr=localhost:8443 -ca cert.pem

test:
	go test ./...

integration:
	@echo "Running integration rate test (this may take a while). To run shorter use: go test -run TestIntegrationRate -short"
	go test -v -run TestIntegrationRate ./internal/server

bench:
	@echo "Build bench runner and run. Use DURATION, WORKERS env or flags"
	go build -o merkle-bench ./cmd/bench
	./merkle-bench -duration 30

clean:
	rm -f merkle-server merkle-client merkle-checker
