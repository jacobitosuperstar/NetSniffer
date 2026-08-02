package main

import (
	"bufio"
	"bytes"
	"net/http"
	"sync"

	"github.com/gopacket/gopacket"
)

// HttpCounter here is where we will store all the traffic. Concurrent mode
// baby
type HttpCounter struct {
	total int
	mu    sync.RWMutex
	hosts map[string]int
}

func NewCounter() *HttpCounter {
	counter := HttpCounter{hosts: make(map[string]int)}
	return &counter
}

// add concurrency safe add operation
func (counter *HttpCounter) add(host string) {
	counter.mu.Lock()
	counter.total += 1
	counter.hosts[host] += 1
	counter.mu.Unlock()
}

// count count the total requests and the map
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

	// now everything is in here
	counter.add(request.Host)
}
