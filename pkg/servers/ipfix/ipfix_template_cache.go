package ipfix

import (
	"sync"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/packet/ipfix"
)

type templateCacheKey struct {
	agent             bnet.IP
	observationDomain uint32
	templateID        uint16
}

func newTemplateCacheKey(agent bnet.IP, observationDomain uint32, templateID uint16) templateCacheKey {
	return templateCacheKey{
		agent:             agent,
		observationDomain: observationDomain,
		templateID:        templateID,
	}
}

type templateCache struct {
	cache map[templateCacheKey]ipfix.TemplateRecords
	lock  sync.RWMutex
}

// newTemplateCache creates and initializes a new `templateCache` instance
func newTemplateCache() *templateCache {
	return &templateCache{
		cache: make(map[templateCacheKey]ipfix.TemplateRecords),
	}
}

func (c *templateCache) set(rtr bnet.IP, domainID uint32, templateID uint16, records ipfix.TemplateRecords) {
	k := newTemplateCacheKey(rtr, domainID, templateID)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.cache[k] = records
}

func (c *templateCache) get(rtr bnet.IP, domainID uint32, templateID uint16) *ipfix.TemplateRecords {
	k := newTemplateCacheKey(rtr, domainID, templateID)

	c.lock.RLock()
	defer c.lock.RUnlock()

	templateRecords, found := c.cache[k]
	if !found {
		return nil
	}

	return &templateRecords
}
