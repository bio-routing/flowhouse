package interfaces

type Interfaces struct {
	devices map[string]*Device
}

func New() *Interfaces {
	return &Interfaces{
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
