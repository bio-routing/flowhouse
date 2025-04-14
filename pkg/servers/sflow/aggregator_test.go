package sflow

import (
	"testing"
	"time"

	"github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	initialTime := time.Now()
	mockedTime := initialTime

	out := make(chan []*flow.Flow, 10)
	agg := newAggregator(out)
	agg.timeNow = func() time.Time { return mockedTime }

	agg.ingest(exampleFlow(t, mockedTime))
	// should have flushed an empty list at the beginning
	select {
	case flows := <-out:
		assert.Empty(t, flows)
	default:
		t.Error("no flows in channel")
	}

	// advance time by 5 seconds
	mockedTime = mockedTime.Add(5 * time.Second)

	agg.ingest(exampleFlow(t, mockedTime))
	assert.Len(t, out, 0) // should not have flushed

	// advance time by 10 seconds
	mockedTime = mockedTime.Add(10 * time.Second)

	agg.ingest(exampleFlow(t, mockedTime))
	assert.Len(t, out, 1) // should have flushed once

	// check flushed flows
	{
		select {
		case flows := <-out:
			require.Len(t, flows, 1)
			fl := flows[0]
			assert.Equal(t,
				initialTime.Add(15*time.Second).Truncate(10*time.Second).Unix(),
				fl.Timestamp,
			)
		default:
			t.Error("no flow available")
		}
	}
}

func must[T any](t testing.TB) func(res T, err error) T {
	return func(res T, err error) T {
		if err != nil {
			t.Error(err)
		}
		return res
	}
}
