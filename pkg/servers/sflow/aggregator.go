package sflow

import (
	"time"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
)

const (
	aggregationWindow = 10 * time.Second
)

type aggregator struct {
	data      map[key]*flow.Flow
	stopCh    chan struct{}
	ingress   chan *flow.Flow
	output    chan []*flow.Flow
	lastFlush time.Time
	timeNow   func() time.Time
}

func newAggregator(output chan []*flow.Flow) *aggregator {
	a := &aggregator{
		data:    make(map[key]*flow.Flow),
		stopCh:  make(chan struct{}),
		ingress: make(chan *flow.Flow),
		output:  output,
		timeNow: time.Now,
	}

	go a.service()
	return a
}

func (a *aggregator) stop() {
	close(a.stopCh)
}

type key struct {
	agent    bnet.IP
	src      bnet.IP
	dst      bnet.IP
	sport    uint16
	dport    uint16
	protocol uint8
}

func flowToKey(fl *flow.Flow) key {
	return key{
		agent:    fl.Agent,
		src:      fl.SrcAddr,
		dst:      fl.DstAddr,
		sport:    fl.SrcPort,
		dport:    fl.DstPort,
		protocol: fl.Protocol,
	}
}

func (a *aggregator) isStopped() bool {
	select {
	case <-a.stopCh:
		return true
	default:
		return false
	}
}

func (a *aggregator) service() {
	for {
		if a.isStopped() {
			return
		}
		fl := <-a.ingress
		a.ingest(fl)
	}
}

func (a *aggregator) ingest(fl *flow.Flow) {
	normalizedIngestTime := a.timeNow().Truncate(aggregationWindow)

	timeSinceLastFlush := normalizedIngestTime.Sub(a.lastFlush)
	if timeSinceLastFlush >= aggregationWindow {
		a.flush()
		a.lastFlush = normalizedIngestTime
	}

	fl.Timestamp = normalizedIngestTime.Unix()
	a.add(fl)
}

func (a *aggregator) add(fl *flow.Flow) {
	k := flowToKey(fl)

	if _, exists := a.data[k]; !exists {
		a.data[k] = fl
		return
	}

	a.data[k].Add(fl)
}

func (a *aggregator) flush() {
	s := make([]*flow.Flow, len(a.data))

	i := 0
	for _, fl := range a.data {
		s[i] = fl
		i++
	}

	a.output <- s
	a.data = make(map[key]*flow.Flow)
}
