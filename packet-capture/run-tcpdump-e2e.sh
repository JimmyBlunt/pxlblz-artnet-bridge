#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROUTER_ROOT="$ROOT/router"
OUT_DIR="${1:-$ROOT/packet-capture/results}"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

ROUTER_BIN="$OUT_DIR/pxlblz-router"
PCAP="$OUT_DIR/installation-known.pcap"
ROUTER_LOG="$OUT_DIR/router.log"
TCPDUMP_TEXT="$OUT_DIR/tcpdump.txt"
VERIFY_JSON="$OUT_DIR/verify.json"

cleanup() {
  set +e
  if [[ -n "${ROUTER_PID:-}" ]]; then kill "$ROUTER_PID" 2>/dev/null || true; fi
  if [[ -n "${TCPDUMP_PID:-}" ]]; then kill "$TCPDUMP_PID" 2>/dev/null || true; fi
  sudo ip addr del 10.0.0.244/32 dev lo 2>/dev/null || true
  sudo ip addr del 10.0.0.253/32 dev lo 2>/dev/null || true
  sudo ip addr del 10.0.0.251/32 dev lo 2>/dev/null || true
}
trap cleanup EXIT

command -v tcpdump >/dev/null
command -v python3 >/dev/null
command -v go >/dev/null
command -v node >/dev/null
command -v tsc >/dev/null

echo "Assigning production controller IPs to loopback..."
for ip in 10.0.0.244 10.0.0.253 10.0.0.251; do
  sudo ip addr add "$ip/32" dev lo 2>/dev/null || true
done

echo "Building router..."
(
  cd "$ROUTER_ROOT"
  go test ./...
  go build -o "$ROUTER_BIN" ./cmd/pxlblz-router
)

echo "Compiling exact TypeScript adapter..."
BUILD_DIR="$OUT_DIR/adapter-build"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"
tsc "$ROOT/pxlblz-integration/src/externalPixelOutput.ts" \
  --target ES2022 --module ES2022 --moduleResolution bundler \
  --lib ES2022,DOM --outDir "$BUILD_DIR"

echo "Starting tcpdump..."
sudo tcpdump -i lo -n -s 0 -U -w "$PCAP" "udp port 6454" >"$OUT_DIR/tcpdump-live.log" 2>&1 &
TCPDUMP_PID=$!
sleep 1

echo "Starting production-route router..."
"$ROUTER_BIN" \
  --config "$ROUTER_ROOT/config/routes.installation-known.json" \
  --input ws \
  --duration 6s \
  >"$ROUTER_LOG" 2>&1 &
ROUTER_PID=$!
sleep 1

echo "Driving exact adapter..."
(
  cd "$ROOT/pxlblz-integration/virtual-test"
  rm -rf build
  mkdir -p build
  cp "$BUILD_DIR/externalPixelOutput.js" build/
  node virtual-pxlblz-driver.mjs --pixels 8186 --fps 60 --seconds 4
)

wait "$ROUTER_PID"
ROUTER_PID=""

sleep 1
sudo kill "$TCPDUMP_PID" 2>/dev/null || true
wait "$TCPDUMP_PID" 2>/dev/null || true
TCPDUMP_PID=""

tcpdump -nn -vv -r "$PCAP" > "$TCPDUMP_TEXT"

python3 "$ROOT/packet-capture/verify_pcap.py" \
  "$PCAP" \
  "$ROUTER_ROOT/config/routes.installation-known.json" \
  "$VERIFY_JSON"

cat "$VERIFY_JSON"
echo
echo "TCPDUMP_ARTNET_PASS"
