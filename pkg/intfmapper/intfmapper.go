package intfmapper

import (
	"fmt"
	"sync"

	bnet "github.com/bio-routing/bio-rd/net"
	log "github.com/sirupsen/logrus"
)

// IntfMapper allows mapping interface IDs into Names and vice versa
type IntfMapper struct {
	devices   map[bnet.IP]*device
	devicesMu sync.RWMutex
}

// New creates a new IntfMapper
func New() *IntfMapper {
	return &IntfMapper{
		devices: make(map[bnet.IP]*device),
	}
}

// Resolve resolves an agents interface ID into is' name
func (im *IntfMapper) Resolve(agent bnet.IP, ifID uint32) string {
	im.devicesMu.RLock()
	defer im.devicesMu.RUnlock()

	if _, exists := im.devices[agent]; !exists {
		log.Warningf("IntfMapper: Device %q not found", agent.String())
		return ""
	}

	return im.devices[agent].resolve(ifID)
}

// AddDevice adds a device
func (im *IntfMapper) AddDevice(addr bnet.IP, community string) error {
	im.devicesMu.Lock()
	defer im.devicesMu.Unlock()

	if _, exists := im.devices[addr]; exists {
		return fmt.Errorf("Device exists already")
	}

	im.devices[addr] = newDevice(addr, community)

	return nil
}
