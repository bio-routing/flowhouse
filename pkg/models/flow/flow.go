package flow

import (
	"fmt"

	bnet "github.com/bio-routing/bio-rd/net"
)

// Flow defines a network flow
type Flow struct {
	Agent      bnet.IP
	TOS        uint8
	SrcPort    uint16
	DstPort    uint16
	SrcAs      uint32
	DstAs      uint32
	NextAs     uint32
	IntIn      string
	IntOut     string
	Packets    uint64
	Protocol   uint8
	Family     uint8
	Timestamp  int64
	Size       uint64
	Samplerate uint64
	SrcAddr    bnet.IP
	DstAddr    bnet.IP
	NextHop    bnet.IP
	SrcPfx     bnet.Prefix
	DstPfx     bnet.Prefix
	VRFIn      uint64
	VRFOut     uint64
}

// Add adds up to flows
func (fl *Flow) Add(a *Flow) {
	fl.Size += a.Size
	fl.Packets += a.Packets
}

// Dump dumps the flow
func (fl *Flow) Dump() {
	fmt.Printf("--------------------------------\n")
	fmt.Printf("Flow dump:\n")
	fmt.Printf("Router: %s\n", fl.Agent.String())
	fmt.Printf("Family: %d\n", fl.Family)
	fmt.Printf("SrcAddr: %s\n", fl.SrcAddr.String())
	fmt.Printf("DstAddr: %s\n", fl.DstAddr.String())
	fmt.Printf("Protocol: %d\n", fl.Protocol)
	fmt.Printf("NextHop: %s\n", fl.NextHop.String())
	fmt.Printf("IntIn: %s\n", fl.IntIn)
	fmt.Printf("IntOut: %s\n", fl.IntOut)
	fmt.Printf("TOS/COS: %d\n", fl.TOS)
	fmt.Printf("Packets: %d\n", fl.Packets)
	fmt.Printf("Bytes: %d\n", fl.Size)
	fmt.Printf("--------------------------------\n")
}
