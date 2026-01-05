#!/bin/bash
echo "Building Interactive Environment..."
docker build -t nam-e2e -f Dockerfile.e2e .

echo ""
echo "=== NAM Interactive Environment ==="
echo "You are about to enter a Docker container with NAM installed."
echo ""
echo "Recommended steps to experience NAM:"
echo "1. Start the fake proxy node (background):"
echo "   sing-box -c /etc/sing-box/config.json &"
echo ""
echo "2. Run the initialization wizard:"
echo "   nam init"
echo ""
echo "3. Start the daemon:"
echo "   nam start --daemon"
echo ""
echo "4. Open the Real-time Monitor (TUI):"
echo "   nam monitor"
echo "   (Press 'q' or 'Esc' to exit monitor)"
echo ""
echo "5. Simulate traffic (open a new terminal inside tmux or background the previous commands):"
echo "   nc -v -z 127.0.0.1 10000"
echo ""
echo "Tools available: vim, nano, tmux, curl, nc"
echo "Entering shell... (Type 'exit' to quit)"
echo "======================================="

# Run interactive shell
docker run -it --rm --cap-add=NET_ADMIN --cap-add=SYS_PTRACE nam-e2e /bin/bash
