.PHONY: local run build

build:
	go build -o NetSniffer ./src

run: build
	sudo ./NetSniffer -log=test.log -seconds=20

local: build
	sudo ./NetSniffer -device lo0 -BPF "tcp dst port 8080" -log /tmp/sniff.log -seconds 20
