package ipfix

import (
	"sync"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/packet/ipfix"
)

type templateCache struct {
	cache map[bnet.IP]map[uint32]map[uint16]ipfix.TemplateRecords
	lock  sync.RWMutex
}

// newTemplateCache creates and initializes a new `templateCache` instance
func newTemplateCache() *templateCache {
	return &templateCache{cache: make(map[bnet.IP]map[uint32]map[uint16]ipfix.TemplateRecords)}
}

func (c *templateCache) set(rtr bnet.IP, domainID uint32, templateID uint16, records ipfix.TemplateRecords) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.cache[rtr]; !ok {
		c.cache[rtr] = make(map[uint32]map[uint16]ipfix.TemplateRecords)
	}

	if _, ok := c.cache[rtr][domainID]; !ok {
		c.cache[rtr][domainID] = make(map[uint16]ipfix.TemplateRecords)
	}

	c.cache[rtr][domainID][templateID] = records
}

func (c *templateCache) get(rtr bnet.IP, domainID uint32, templateID uint16) *ipfix.TemplateRecords {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if _, ok := c.cache[rtr]; !ok {
		return nil
	}

	if _, ok := c.cache[rtr][domainID]; !ok {
		return nil
	}

	if _, ok := c.cache[rtr][domainID][templateID]; !ok {
		return nil
	}

	ret := c.cache[rtr][domainID][templateID]
	return &ret
}
