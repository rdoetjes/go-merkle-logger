# Makefile for merkle logging service

.PHONY: all build server client checker cert run-server run-client update-sbom clean

all: build

build: proto
	@echo "building binaries (proto files generated)"
	go build -o merkle-server ./cmd/server
	go build -o merkle-client ./cmd/client
	go build -o merkle-checker ./cmd/checker
	go build -o merkle-bench ./cmd/bench

proto: install-proto-tools
	@echo "generating go files from proto/**/*.proto"
	# Clean any previously generated stray files that can confuse imports
	@rm -rf phonax.com merkle/logging || true
	# Generate into the proto/ directory using source-relative paths so generated files are placed under proto/...
	# Use helper script to generate protos robustly
	@chmod +x scripts/generate_protos.sh
	@./scripts/generate_protos.sh

install-proto-tools:
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc into $(go env GOPATH)/bin or $(go env GOBIN)"
	@echo "This may take a while the first time."
	# Use pinned versions compatible with Go 1.20 runners
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	@echo "installed protoc plugins."

server:
	go build -o merkle-server ./cmd/server

client:
	go build -o merkle-client ./cmd/client

checker:
	go build -o merkle-checker ./cmd/checker

cert:
	@echo "Generating CA, Server, and Client certificates..."
	# Generate CA
	openssl genrsa -out ca-key.pem 2048
	openssl req -new -x509 -days 365 -key ca-key.pem -out ca.pem -subj "/CN=Merkle CA"

	# Generate Server Cert
	openssl genrsa -out server-key.pem 2048
	openssl req -new -key server-key.pem -out server.csr -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
	openssl x509 -req -days 365 -in server.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -copy_extensions copyall

	# Generate Client Cert
	openssl genrsa -out client-key.pem 2048
	openssl req -new -key client-key.pem -out client.csr -subj "/CN=merkle-client"
	openssl x509 -req -days 365 -in client.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem

run-server:
	@echo "Starting server with mTLS and ACL. Use MERKLE_HMAC_KEY and MERKLE_ALLOWED_CNS env vars to configure."
	export MERKLE_HMAC_KEY=$${MERKLE_HMAC_KEY:-demo-key}; \
	export MERKLE_ALLOWED_CNS=$${MERKLE_ALLOWED_CNS:-merkle-client}; \
	./merkle-server -tls-cert=server-cert.pem -tls-key=server-key.pem -ca=ca.pem -addr=:8443 -backend=file -logfile=./protected.log

run-client:
	./merkle-client -addr=localhost:8443 -ca ca.pem -tls-cert=client-cert.pem -tls-key=client-key.pem

test: proto
	@echo "running tests (no cache)"
	go test -v -count=1 ./...

integration:
	@echo "Running integration rate test (this may take a while). To run shorter use: go test -run TestIntegrationRate -short"
	go test -v -run TestIntegrationRate ./internal/server

bench:
	@echo "Build bench runner and run. Use DURATION, WORKERS env or flags"
	go build -o merkle-bench ./cmd/bench
	./merkle-bench -duration 30

update-sbom:
	@echo "Updating dependencies and generating SBOM..."
	go mod tidy
	go get -u ./...
	go mod tidy
	@# Scan current directory and output to file explicitly
	syft scan . --output cyclonedx-json@1.5=sbom.cdx.json
	@echo "Running vulnerability scan..."
	grype sbom.cdx.json

clean:
	rm -f merkle-server merkle-client merkle-checker merkle-bench
	rm -f *.pem *.csr *.srl
