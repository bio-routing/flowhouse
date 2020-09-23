package flow

import (
	bnet "github.com/bio-routing/bio-rd/net"
)

// Flow defines a network flow
type Flow struct {
	Agent      bnet.IP
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
