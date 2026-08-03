#!/usr/bin/env bash
# test_local.sh — deterministic NetSniffer test against a LOCAL server.
# No internet, no redirects, no timeouts: the same counts every single run.
#
#   Terminal A:  python3 local_server.py 8080
#   Terminal B:  sudo ./NetSniffer -i lo0 -f "tcp dst port 8080" -log /tmp/sniff.log -seconds 15
#   Terminal C:  bash test_local.sh
#
# The sniffer keys the histogram on the HTTP Host header, not the IP, so we
# synthesize distinct hosts with `-H "Host: ..."` against the one local server.
# That makes the expected top-N exact — including the tie-breaks.
set -u

PORT="${1:-8080}"
BASE="http://127.0.0.1:${PORT}"
UA="NetSnifferLocal/1.0"

# --- expected result (what the sniffer histogram should show) ------------------
#   alpha.local    5
#   bravo.local    3   (ties with echo at 3 -> ranks ABOVE it, name ascending)
#   echo.local     3   (keep-alive: 3 requests over ONE connection)
#   charlie.local  2
#   delta.local    1   (ties with foxtrot at 1 -> ranks above, name ascending)
#   foxtrot.local  1   (the big POST)
#   TOTAL          15
cat <<'EXPECTED'
Expected sniffer output:
  Total: 15
  1 alpha.local    5
  2 bravo.local    3
  3 echo.local     3
  4 charlie.local  2
  5 delta.local    1
  6 foxtrot.local  1
EXPECTED
echo

# 11 single GETs, distribution 5/3/2/1 across four hosts.
hosts=(
  alpha.local alpha.local alpha.local alpha.local alpha.local
  bravo.local bravo.local bravo.local
  charlie.local charlie.local
  delta.local
)

echo "Firing ${#hosts[@]} single GETs + keep-alive x3 + 1 big POST at ${BASE} ..."
echo

pids=()
for h in "${hosts[@]}"; do
  curl --silent --show-error --http1.1 -A "$UA" --max-time 5 \
       -o /dev/null -H "Host: ${h}" \
       -w "[%{http_code}] ${h} (${h}) %{num_connects} new-conn\n" \
       "${BASE}/" &
  pids+=($!)
done

# SPECIAL 1 — keep-alive: 3 requests over ONE connection, all Host: echo.local.
# The sniffer should count 3 request boundaries; curl should report 1 new-conn.
curl --silent --show-error --http1.1 -A "$UA" --max-time 5 \
     -H "Host: echo.local" \
     -o /dev/null "${BASE}/a" \
     -o /dev/null "${BASE}/b" \
     -o /dev/null "${BASE}/c" \
     -w "[keep-alive x3 -> echo.local] new-conns=%{num_connects} (expect 1)\n" &
pids+=($!)

# SPECIAL 2 — big POST (~200 KB, Host: foxtrot.local). Loopback MTU is large
# (~16 KB on macOS), so 200 KB forces ~13 TCP segments the reassembler must
# stitch in order before http.ReadRequest can consume the Content-Length body.
# A real temp file (not a pipe) makes curl send a clean Content-Length, not chunked.
payload="$(mktemp)"
trap 'rm -f "$payload"' EXIT
yes A | head -c 200000 > "$payload"

curl --silent --show-error --http1.1 -A "$UA" --max-time 5 \
     -H "Host: foxtrot.local" \
     --request POST --data-binary @"$payload" \
     -o /dev/null "${BASE}/upload" \
     -w "[big POST -> foxtrot.local] uploaded=%{size_upload}B new-conns=%{num_connects}\n" &
pids+=($!)

wait
echo
echo "Sent $(( ${#hosts[@]} + 3 + 1 )) requests total. Compare against the Expected block above."
