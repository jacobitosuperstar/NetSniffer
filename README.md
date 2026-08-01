# NetSniffer

As in the PDF the main thing that we need to create is a Network sniffer that
allows us to check the behaviour of the network.

Main points to deliver:

1. Total number of http request detected.
2. Histogram of the top 10 requested hosts by number of queries descending.

## Network

There are 4 main layer on which we can watch the traffic on a device

1. Application Level
2. Socket Level (netstat)
3. TCP/IP Level
4. Driver (BPF)

For this implementation we will be working on the BPF (Berkley Packet Filter)
layer, where we will capture the packets and check the source of them.

## Traffict Classification

When checking the behaviour of a network, there are two fundamental issues, one
being the amount of connections that we are making, and the other, is the size
of the packets that we are interchanging in the network.

To solve this, and not to overflow the memory, what we will do is to have 3
rings.

- Ring for the amount of connections that we are having to an IP address
  - For this ring we will apply a frequency based eviction, that way stale
    conntections in the time window don't hide frequent connections that don't
    match the lowest connection ammount. -> singleton connection overflow
    doesn't happen.
- Ring for the frequency of connections that we are having an IP address
  - We want to find connection ammount in a time window. There could be
    attackers that do low frequency high burst of connection attacks. We see
    them here.
- Ring for the size of the packets being send bidirectionally (uploads and
  downloads)
  - There could be a connection that is low frequency, and low connection
    ammount but sends and receives big packets.

that way we will have a full scope of what is actually happening in the
network.

## BPF

## Limitations

I have not been able to figure out how to capture, low frequency, low
connections, and low packet size traffic (these are normally life signals from
malware to the attacker host)

We won't be counting HTTP request. We will pivot to counting through the SNI as
the HTTP host gets missing when we use HTTPS. We will count server handshakes.
