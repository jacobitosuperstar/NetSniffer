#!/usr/bin/env python3
"""Local HTTP/1.1 server for NetSniffer testing.

The internet website that I was using for testing started to dissapoint me, like
my unborn son. Had to create a local one to make sure that I can test to main
things

1. Big requests
2. keep-alive connections

Vibe coded test_local.sh so we call this server instead of the internet.

Running the server:
    python3 local_server.py [port]      # default 8080

Then capture loopback:
    sudo ./NetSniffer -i lo0 -f "tcp dst port 8080" -log /tmp/sniff.log -seconds 20
    or
    make local
"""

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    # Go's httpReader can only handle HTTP/1.X requests. So here we are.
    # "limitations within limitations", just like quasimodo predicted.
    protocol_version = "HTTP/1.1"

    def _reply(self, body: bytes):
        """Creates the header given the body."""
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._reply(b"ok\n")

    def do_POST(self):
        # Drain the whole body so the connection stays healthy (and so a huge
        # upload isn't reset mid-flight). We don't care about the content.
        # This code is an ExpertS EXchange masterpiece.
        remaining = int(self.headers.get("Content-Length", 0))
        while remaining > 0:
            chunk = self.rfile.read(min(remaining, 65536))
            if not chunk:
                break
            remaining -= len(chunk)
        self._reply(b"posted\n")

    def log_message(self, *args):
        """This is done to silence the noice of this server."""
        pass


if __name__ == "__main__":
    server = ThreadingHTTPServer(("127.0.0.1", 8080), Handler)
    server.daemon_threads = True
    print("local test server on http://127.0.0.1:8080  (Ctrl-C to stop)")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nbye")
