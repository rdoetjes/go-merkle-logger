#!/bin/bash
set -e

# Build latest changes
make cert
make build

echo "1. Testing with CORRECT Common Name..."
./merkle-server -tls-cert=server-cert.pem -tls-key=server-key.pem -ca=ca.pem -allowed-cn=merkle-client -addr=:8446 -backend=file -logfile=./test.log &
SERVER_PID=$!
sleep 2

if ./merkle-client -addr=localhost:8446 -ca ca.pem -tls-cert=client-cert.pem -tls-key=client-key.pem; then
    echo "Success: Authorized CN allowed."
else
    echo "Failure: Authorized CN was denied."
    kill $SERVER_PID
    exit 1
fi
kill $SERVER_PID
sleep 1

echo "2. Testing with WRONG Common Name (ACL Check)..."
./merkle-server -tls-cert=server-cert.pem -tls-key=server-key.pem -ca=ca.pem -allowed-cn=wrong-client -addr=:8446 -backend=file -logfile=./test.log &
SERVER_PID=$!
sleep 2

if ./merkle-client -addr=localhost:8446 -ca ca.pem -tls-cert=client-cert.pem -tls-key=client-key.pem; then
    echo "Failure: Unauthorized CN was allowed."
    kill $SERVER_PID
    exit 1
else
    echo "Success: Unauthorized CN was blocked by ACL."
fi
kill $SERVER_PID

echo "ACL test passed!"
