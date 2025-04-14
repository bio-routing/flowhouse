package sflow

import (
	"testing"
	"time"

	"github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/stretchr/testify/assert"
)

func exampleFlow(t testing.TB, ts time.Time) *flow.Flow {
	return &flow.Flow{
		Agent:     must[net.IP](t)(net.IPFromString("2001:db8::1")),
		SrcPort:   34567,
		DstPort:   443,
		Packets:   10,
		Protocol:  6,
		Family:    4,
		Timestamp: ts.Unix(),
		Size:      200,
		SrcAddr:   must[net.IP](t)(net.IPFromString("198.51.100.24")),
		DstAddr:   must[net.IP](t)(net.IPFromString("203.0.113.30")),
	}
}

func TestAggregatorBuffering(t *testing.T) {
	mockedTime := time.Now()

	out := make(chan []*flow.Flow, 10)
	agg := newAggregator(out)
	agg.timeNow = func() time.Time { return mockedTime }

	agg.ingest(exampleFlow(t, mockedTime))
	assert.Len(t, out, 0) // should not have flushed

	// advance time by 2 seconds
	mockedTime = mockedTime.Add(2 * time.Second)

	agg.ingest(exampleFlow(t, mockedTime))
	assert.Len(t, out, 0) // should not have flushed

	// advance time by 10 seconds
	agg.ingest(exampleFlow(t, mockedTime))
	assert.Len(t, out, 1) // should have flushed one record
}

func must[T any](t testing.TB) func(res T, err error) T {
	return func(res T, err error) T {
		if err != nil {
			t.Error(err)
		}
		return res
	}
}
