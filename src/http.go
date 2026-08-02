package main

import (
	"bufio"
	"bytes"
	"net/http"

	"github.com/gopacket/gopacket"
)

// HttpCounter here is where we will store all the traffic.
type HttpCounter struct {
	total int
	hosts map[string]int
}

func NewCounter() *HttpCounter {
	counter := HttpCounter{0, make(map[string]int)}
	return &counter
}

func (counter *HttpCounter) count(pkt gopacket.Packet) {
	appLayer := pkt.ApplicationLayer()
	// the packet is not in this application layer
	if appLayer == nil {
		return
	}

	payload := appLayer.Payload()
	// there is no payload
	if len(payload) == 0 {
		return
	}

	request, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(payload)))
	// not a complete http request
	if err != nil {
		return
	}

	// if the request came to here MUST be valid, don't you think??
	counter.total += 1

	// not send to a host
	if request.Host == "" {
		counter.hosts["Unkown"] += 1
		return
	}
	counter.hosts[request.Host] += 1
}
