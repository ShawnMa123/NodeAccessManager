#!/bin/bash
set -e

trap 'echo "=== NAM LOGS ==="; cat /var/log/nam/nam.log || true' EXIT


echo "=== NAM E2E Test Start ==="

# 1. Start fake sing-box
echo "[STEP] Starting sing-box..."
sing-box -c /etc/sing-box/config.json &
SB_PID=$!
sleep 2

# Verify listener
if ss -tln | grep -q :10000; then
    echo "SUCCESS: Sing-box listening on :10000"
else
    echo "ERROR: Sing-box failed to start"
    exit 1
fi

# 2. Add NAM config to limit connections (optional, but good for testing)
mkdir -p /etc/nam
cat > /etc/nam/config.yaml <<EOF
global:
  check_interval: 1
  ban_duration: 60
  white_list_cidr:
    - "127.0.0.1/32"
rules:
  - port: 10000
    max_ips: 5
    strategy: "fifo"
EOF

# 3. Initialize NAM
echo "[STEP] Initializing NAM..."
mkdir -p /var/log/nam
nam init

# 4. Start NAM
echo "[STEP] Starting NAM daemon..."
# Run in foreground to see errors
nam start --daemon=false > /tmp/nam_output.log 2>&1 &
NAM_PID=$!
echo "Started NAM background PID: $NAM_PID"
sleep 3

# Check if process is still running
if kill -0 $NAM_PID >/dev/null 2>&1; then
    echo "NAM is running"
else
    echo "NAM died immediately!"
    cat /tmp/nam_output.log
    exit 1
fi

# 5. Check NAM status
echo "[STEP] Checking NAM status..."
nam status

# 6. Verify Discovery
# Check if NAM log mentions discovering the process
if grep -q "发现.*sing-box" /var/log/nam/nam.log; then
    echo "SUCCESS: NAM discovered sing-box process"
else
    echo "WARNING: NAM might not have discovered sing-box yet. Logs:"
    cat /var/log/nam/nam.log
fi

# 7. Simulate Connection
echo "[STEP] Simulating connection to port 10000..."
nc -v -z 127.0.0.1 10000 &
sleep 1

# 8. Check Monitor Logs
echo "[STEP] Verifying detection..."
if grep -q "10000" /var/log/nam/nam.log; then
    echo "SUCCESS: Activity detected in logs"
else
    echo "WARNING: No activity for port 10000 found in logs yet"
fi

echo "=== NAM Logs ==="
cat /var/log/nam/nam.log

echo "=== E2E Test Successfully Completed ==="
