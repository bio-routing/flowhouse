package intfmapper

import "sync"

type IntfMapper struct {
	devices   map[string]*Device
	devicesMu sync.RWMutex
}

// New creates a new IntfMapper
func New() *IntfMapper {
	return &IntfMapper{
		devices: make(map[string]*Device),
	}
}

type Device struct {
	interfacesByID   map[uint32]*Interface
	interfaceyByName map[string]*Interface
}

type Interface struct {
	ID   uint32
	Name string
}

func (d *Device) addInterface(ifa *Interface) {
	d.interfacesByID[ifa.ID] = ifa
	d.interfaceyByName[ifa.Name] = ifa
}

// GetNameByID gets an interfaces ID by its name
func (im *IntfMapper) GetNameByID(deviceName string, ifID uint32) string {
	if _, exists := im.devices[deviceName]; !exists {
		return ""
	}

	ifa, exists := im.devices[deviceName].interfacesByID[ifID]
	if !exists {
		return ""
	}

	return ifa.Name
}

// GetIDByName gets an interfaces name by its ID
func (im *IntfMapper) GetIDByName(deviceName string, ifName string) uint32 {
	if _, exists := im.devices[deviceName]; !exists {
		return 0
	}

	ifa, exists := im.devices[deviceName].interfaceyByName[ifName]
	if !exists {
		return 0
	}

	return ifa.ID
}
