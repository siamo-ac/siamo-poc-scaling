#!/usr/bin/env bash
# End-to-end demo of the scaling POC.
# Usage: ./scripts/demo.sh
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/workspace/toolchains/go/bin"

echo "==> building..."
go build -o /tmp/scaling-backend ./cmd/backend
go build -o /tmp/scaling-balancer ./cmd/balancer

echo "==> starting 3 backends..."
/tmp/scaling-backend -addr 127.0.0.1:9001 -id backend-1 >/tmp/sb1.log 2>&1 &
P1=$!
/tmp/scaling-backend -addr 127.0.0.1:9002 -id backend-2 >/tmp/sb2.log 2>&1 &
P2=$!
/tmp/scaling-backend -addr 127.0.0.1:9003 -id backend-3 >/tmp/sb3.log 2>&1 &
P3=$!
echo "==> starting balancer on 18080..."
/tmp/scaling-balancer -addr 127.0.0.1:18080 \
  -backends "http://127.0.0.1:9001,http://127.0.0.1:9002,http://127.0.0.1:9003" >/tmp/slb.log 2>&1 &
LB=$!
trap 'kill $P1 $P2 $P3 $LB 2>/dev/null || true' EXIT
sleep 1

echo "==> 1. hammer the balancer 12 times (round-robin):"
for i in $(seq 1 12); do
  curl -s -D - -o /dev/null http://127.0.0.1:18080/work | grep -i 'X-Backend-Id'
done | sort | uniq -c

echo "==> 2. distribution according to the balancer:"
curl -s http://127.0.0.1:18080/status; echo

echo "==> 3. sticky routing: same key twice -> same backend"
for i in 1 2; do
  curl -s -o /dev/null -D - "http://127.0.0.1:18080/route?key=alice" | grep -i 'X-Backend-Id'
done
echo "==> 4. different keys spread across backends:"
for k in alice bob carol dave erin frank; do
  printf "%-6s -> " "$k"
  curl -s -o /dev/null -D - "http://127.0.0.1:18080/route?key=$k" | grep -i 'X-Backend-Id' | tr -d '\r'
done

echo
echo "Demo complete. Try scaling out: start a 4th backend and restart the balancer with 4 URLs."
