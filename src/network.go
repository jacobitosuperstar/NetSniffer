package main

import (
	"fmt"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcap"
)

// pkt_capture opens the given device for live capture and then we apply the BPF
// filter. This returns a channel of packets with the handle.
func pkt_capture(
	device string,
	filter string,
) (
	<-chan gopacket.Packet,
	*pcap.Handle,
	error,
) {
	const (
		snaplen     = 1600  // 1518 is the standard ethernet frame
		promiscuous = false // we only care about traffic in our device
	)

	// We are the ones that close the handle
	handle, err := pcap.OpenLive(device, snaplen, promiscuous, pcap.BlockForever)

	if err != nil {
		return nil, nil, fmt.Errorf("Error opening the device %s: %w", device, err)
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		handle.Close()
		return nil, nil, fmt.Errorf("Error setting the BPF %s: %w", filter, err)
	}

	// PacketSource -> decoding options for the received packets.
	source := gopacket.NewPacketSource(handle, handle.LinkType())
	return source.Packets(), handle, nil
}
