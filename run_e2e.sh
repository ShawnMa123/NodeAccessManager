#!/bin/bash
echo "Building E2E image..."
docker build -t nam-e2e -f Dockerfile.e2e .

echo "Running E2E test..."
# --cap-add=NET_ADMIN is required for iptables and ss -K
# --cap-add=SYS_PTRACE might be needed for reading /proc (though often not strictly required in unconfined, let's add it to be safe)
docker run --rm --cap-add=NET_ADMIN --cap-add=SYS_PTRACE nam-e2e
