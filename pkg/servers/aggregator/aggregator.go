package aggregator

import (
	"time"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
)

const (
	aggregationWindow = 10 * time.Second
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
	data      map[Key]*flow.Flow
	stopCh    chan struct{}
	ingress   chan *flow.Flow
	output    chan []*flow.Flow
	lastFlush time.Time
	timeNow   func() time.Time
}

func New(output chan []*flow.Flow) *Aggregator {
	a := &Aggregator{
		data:    make(map[Key]*flow.Flow),
		stopCh:  make(chan struct{}),
		ingress: make(chan *flow.Flow),
		output:  output,
		timeNow: time.Now,
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
	normalizedIngestTime := a.timeNow().Truncate(aggregationWindow)

	timeSinceLastFlush := normalizedIngestTime.Sub(a.lastFlush)
	if timeSinceLastFlush >= aggregationWindow {
		a.flush()
		a.lastFlush = normalizedIngestTime
	}

	fl.Timestamp = normalizedIngestTime.Unix()
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
