package ipfix

import (
	"sync"

	bnet "github.com/bio-routing/bio-rd/net"
)

type sampleRateCacheKey struct {
	agent               bnet.IP
	observationDomainID uint32
}

func newSampleRateCacheKey(agent bnet.IP, observationDomainID uint32) sampleRateCacheKey {
	return sampleRateCacheKey{
		agent:               agent,
		observationDomainID: observationDomainID,
	}
}

type sampleRateCache struct {
	data   map[sampleRateCacheKey]uint32
	dataMu sync.RWMutex
}

func newSampleRateCache() *sampleRateCache {
	return &sampleRateCache{
		data: make(map[sampleRateCacheKey]uint32),
	}
}

func (src *sampleRateCache) get(agent bnet.IP, observationDomainID uint32) uint32 {
	src.dataMu.RLock()
	defer src.dataMu.RUnlock()

	return src.data[newSampleRateCacheKey(agent, observationDomainID)]
}

func (src *sampleRateCache) set(agent bnet.IP, observationDomainID uint32, rate uint32) {
	src.dataMu.Lock()
	defer src.dataMu.Unlock()

	src.data[newSampleRateCacheKey(agent, observationDomainID)] = rate
}
