#!/usr/bin/env sh
set -euo pipefail

# Find proto files
PROTOS=$(find proto -name "*.proto" -print)
if [ -z "${PROTOS}" ]; then
  echo "no proto files found, skipping"
  exit 0
fi

echo "protoc will run on:"
for p in ${PROTOS}; do
  echo "  $p"
done

# Generate with source_relative into proto/
for p in ${PROTOS}; do
  protoc -I proto --go_out=paths=source_relative:./proto --go-grpc_out=paths=source_relative:./proto "${p}"
done

# Move generated pb.go from nested directories (if any) to proto/ and remove empty dirs
if [ -d "proto/merkle/logging" ]; then
  mv proto/merkle/logging/*.pb.go proto/ || true
  rm -rf proto/merkle
fi

echo "proto generation complete"
