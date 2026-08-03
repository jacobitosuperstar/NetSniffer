package main

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sync"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/tcpassembly"
	"github.com/gopacket/gopacket/tcpassembly/tcpreader"
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

// Add concurrency safe add operation
func (counter *HttpCounter) Add(host string) {
	counter.mu.Lock()
	defer counter.mu.Unlock()

	counter.total += 1
	counter.hosts[host] += 1
}

// View councurrency safe state view of the object
func (counter *HttpCounter) View() (int, map[string]int) {
	counter.mu.RLock()
	defer counter.mu.RUnlock()
	total := counter.total
	hosts := maps.Clone(counter.hosts)
	return total, hosts
}

// HostCount stores the information from the hosts map. We will use this to
// arrange the map
type HostCount struct {
	Host  string
	Count int
}

// ranking return both the total http connections and the top 10 host/conn in
// reverse order from
func (counter *HttpCounter) Ranking() (int, []HostCount) {
	total, hosts := counter.View()

	rank := make([]HostCount, 0, len(hosts))
	for host, n := range hosts {
		rank = append(rank, HostCount{Host: host, Count: n})
	}

	// compare(a,b) is ascending, compare(b, a) is descending
	slices.SortFunc(rank, func(a, b HostCount) int {
		// descending for counter
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		// if the value is the same, ascending by name
		return cmp.Compare(a.Host, b.Host)
	})

	// taking the first 10
	n := min(10, len(rank))
	rank = rank[:n]
	return total, rank
}

// PrettyRanking just making everything pretty
func (counter *HttpCounter) PrettyRanking(w io.Writer) {
	total, hosts := counter.Ranking()

	fmt.Fprintf(w, "\nTotal of HTTP 1.X requests: %d\n", total)

	fmt.Fprintln(w, "Rank\tHost\tRequests")

	for i := 0; i < len(hosts); i++ {
		fmt.Fprintf(w, "%d\t%s\t%d\n", i+1, hosts[i].Host, hosts[i].Count)
	}
}

// ConsumerStream Called by the reader. Here we will create the byte stream
// that we will parse and pull the http request out of.
type ConsumerStream struct {
	counter *HttpCounter
	reader  tcpreader.ReaderStream
}

// register function that lives for the lifetime of a single TCP connection by
// (sourceIP, destionationIP), (sourcePort, DestinationPort). Takes that unique
// tcp.ReaderStream and from it we read the outgoing Request.
func (c_stream *ConsumerStream) register() {
	buffer := bufio.NewReader(&c_stream.reader)

	for {
		// TODO @jacobo: fuck me and throw me to the ocean. limitation, this
		// only works for http 1.X
		request, err := http.ReadRequest(buffer)
		if err == io.EOF {
			return
		}
		if err != nil {
			// DEADLOCK
			// reading the tcp.ReaderStream is a blocking operation. If we don't
			// discard everything and continue as fast as possible, we are
			// blocking the main packet assembler to hand us more bytes.
			//
			// We continue taking out bytes here but we are no longer processing
			// them. We need to continue this until we finish all the bytes
			// from the non request or invalid process
			tcpreader.DiscardBytesToEOF(&c_stream.reader)
			return
		}

		// we will discard the request object, better to store somewhere the
		// host name.
		host := request.Host

		// TODO @jacobo: I don't know if we would like to track the size of the
		// bodies. But here is where we would pick up the body size.
		//
		// We drain the body to make sure that we start parsing again it the
		// next request start.
		io.Copy(io.Discard, request.Body)
		request.Body.Close()

		// COUNT ME IN!!!!!!
		c_stream.counter.Add(host)
	}

}

// AssemblerStreamFactory Called by the assempler. here we will capture all the
// created streams for the different http request that we are forming. We will
// collect all of those go routines in a wait group to handle them correctly at
// shutdown of the program.
type AssemblerStreamFactory struct {
	counter *HttpCounter
	wg      *sync.WaitGroup
}

// New we create a new consumer that will reciver all the data from the
// assembler stream and register the http call. AssembleStreamFactory is a
// StreamFactory interface, so it must have a New() method applied to it.
func (asf *AssemblerStreamFactory) New(
	netFlow,
	tcpFlow gopacket.Flow,
) tcpassembly.Stream {
	c_stream := &ConsumerStream{
		counter: asf.counter,
		reader:  tcpreader.NewReaderStream(),
	}
	asf.wg.Go(
		func() {
			c_stream.register()
		},
	)
	return &c_stream.reader
}
