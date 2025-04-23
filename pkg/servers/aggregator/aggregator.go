package aggregator

import (
	"time"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
)

const (
	AggregationWindowSeconds = 10
)

type Key struct {
	Agent    bnet.IP
	Src      bnet.IP
	Dst      bnet.IP
	Sport    uint16
	Dport    uint16
	Protocol uint8
}

type Aggregator struct {
	data                   map[Key]*flow.Flow
	stopCh                 chan struct{}
	ingress                chan *flow.Flow
	output                 chan []*flow.Flow
	currentUnixTimeSeconds int64
}

func New(output chan []*flow.Flow) *Aggregator {
	a := &Aggregator{
		data:    make(map[Key]*flow.Flow),
		stopCh:  make(chan struct{}),
		ingress: make(chan *flow.Flow),
		output:  output,
	}

	go a.service()
	return a
}

func (a *Aggregator) Stop() {
	close(a.stopCh)
}

func FlowToKey(fl *flow.Flow) Key {
	return Key{
		Agent:    fl.Agent,
		Src:      fl.SrcAddr,
		Dst:      fl.DstAddr,
		Sport:    fl.SrcPort,
		Dport:    fl.DstPort,
		Protocol: fl.Protocol,
	}
}

func (a *Aggregator) IsStopped() bool {
	select {
	case <-a.stopCh:
		return true
	default:
		return false
	}
}

func (a *Aggregator) service() {
	for {
		if a.IsStopped() {
			return
		}

		fl := <-a.ingress
		a.Ingest(fl)

	}
}

func (a *Aggregator) Ingest(fl *flow.Flow) {
	currentUnixTimeSeconds := time.Now().Unix()
	currentUnixTimeSeconds -= currentUnixTimeSeconds % AggregationWindowSeconds

	if a.currentUnixTimeSeconds < currentUnixTimeSeconds {
		a.flush()
		a.currentUnixTimeSeconds = currentUnixTimeSeconds
	}

	fl.Timestamp = currentUnixTimeSeconds
	a.add(fl)
}

func (a *Aggregator) add(fl *flow.Flow) {
	k := FlowToKey(fl)

	if _, exists := a.data[k]; !exists {
		a.data[k] = fl
		return
	}

	a.data[k].Add(fl)
}

func (a *Aggregator) GetIngress() chan<- *flow.Flow {
	return a.ingress
}

func (a *Aggregator) flush() {
	s := make([]*flow.Flow, len(a.data))

	i := 0
	for _, fl := range a.data {
		s[i] = fl
		i++
	}

	a.output <- s
	a.data = make(map[Key]*flow.Flow)
}
