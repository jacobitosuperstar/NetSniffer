#!/usr/bin/env bash
# generate_traffic.sh — fire ~20 concurrent cleartext-HTTP requests at a mix of
# hosts to exercise NetSniffer's capture loop + TCP reassembler.
# Test harness only — not part of the product.
#
# Usage: start the sniffer first, e.g.
#   sudo ./NetSniffer -i en1 -seconds 20 -log /tmp/sniff.log
# then run this in another terminal.
set -u

UA="NetSnifferTest/1.0"

# Shared curl config: force HTTP/1.1 (what your http.ReadRequest path parses),
# discard bodies, and print a line the sniffer output can be cross-checked against.
# %{remote_ip}    -> the dest IP your capture keys on (SNI-less, cleartext path)
# %{num_connects} -> 0 means the TCP connection was REUSED (keep-alive)
CURL=(curl --silent --show-error --http1.1 --user-agent "$UA"
      --connect-timeout 8 --max-time 20 --output /dev/null
      --write-out "[%{http_code}] %{url_effective} -> %{remote_ip} (%{size_download}B, %{num_connects} new-conn, %{time_total}s)\n")

# 18 single GETs. Repeated hosts build a non-trivial histogram:
#   codeberg.page x3, jacobobedoya x2, neverssl x2, example.com x2, httpbin x3.
requests=(
  # --- your sites ---
  "http://jacobobedoya.com"
  "http://jacobobedoya.com"
  "http://jacobitosuperstar.codeberg.page/DependaMan/en/"
  "http://jacobitosuperstar.codeberg.page/HandyMan/en/"
  "http://jacobitosuperstar.codeberg.page/"
  # --- sites that stay on cleartext HTTP (won't shove you to HTTPS) ---
  "http://neverssl.com"
  "http://neverssl.com"
  "http://httpforever.com"
  "http://info.cern.ch"
  "http://example.com"
  "http://example.com"
  "http://example.org"
  "http://example.net"
  "http://www.gnu.org"
  "http://ftp.gnu.org/gnu/bash/"
  # --- httpbin: varied bodies ---
  "http://httpbin.org/get"
  "http://httpbin.org/headers"
  "http://httpbin.org/bytes/40000"   # 40 KB response body
)

echo "Firing ${#requests[@]} single requests + 2 special cases (keep-alive, big POST)..."
echo "(watch %{num_connects} and compare hosts/IPs against the sniffer output)"
echo

pids=()
for u in "${requests[@]}"; do
  "${CURL[@]}" "$u" &
  pids+=($!)
done

# SPECIAL 1 — keep-alive: 3 requests over ONE TCP connection. The
# assembler counts request *boundaries* (http.ReadRequest loop), not connections.
# Expect: 3 requests parsed, but only 1 new connection (num_connects=1 on the
# first sub-request, 0 after).
curl --silent --show-error --http1.1 -A "$UA" \
     -o /dev/null http://httpbin.org/get \
     -o /dev/null http://httpbin.org/user-agent \
     -o /dev/null http://httpbin.org/ip \
     -w "[keep-alive x3] last=%{url_effective} total-new-conns=%{num_connects}\n" &
pids+=($!)

# SPECIAL 2 — big POST body (~50 KB). Exercises reassembly on the client->server
# (request) stream: the body spans multiple TCP segments that must be stitched in
# sequence order before http.ReadRequest can consume the Content-Length bytes.
"${CURL[@]}" --request POST --data-binary @<(yes A | head -c 50000) \
     http://httpbin.org/post &
pids+=($!)

wait
echo
echo "All ${#pids[@]} concurrent jobs finished."
