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
	cache map[templateCacheKey]*templateCacheEntry
	lock  sync.RWMutex
}

type templateCacheEntry struct {
	isOptionsTemplate bool
	records           []*ipfix.TemplateRecord
}

// newTemplateCache creates and initializes a new `templateCache` instance
func newTemplateCache() *templateCache {
	return &templateCache{
		cache: make(map[templateCacheKey]*templateCacheEntry),
	}
}

func (c *templateCache) set(rtr bnet.IP, domainID uint32, templateID uint16, records []*ipfix.TemplateRecord, opts bool) {
	k := newTemplateCacheKey(rtr, domainID, templateID)
	v := &templateCacheEntry{
		isOptionsTemplate: opts,
		records:           records,
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.cache[k] = v
}

func (c *templateCache) get(rtr bnet.IP, domainID uint32, templateID uint16) ([]*ipfix.TemplateRecord, bool) {
	k := newTemplateCacheKey(rtr, domainID, templateID)

	c.lock.RLock()
	defer c.lock.RUnlock()

	e, found := c.cache[k]
	if !found {
		return nil, false
	}

	return e.records, e.isOptionsTemplate
}
