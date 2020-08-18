package ifnamecollector

import "strings"

type netInterface struct {
	ID              uint32
	Name            string
	RemoteDevice    string
	RemoteInterface string
	RoutingInstance string
	Role            string
}

func newNetInterface(id uint32, descr string) *netInterface {
	ret := &netInterface{
		ID: id,
	}

	ret.loadDescription(descr)
	return ret
}

func (nif *netInterface) loadDescription(descr string) {
	for _, pair := range strings.Split(descr, ",") {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "rdev":
			nif.RemoteDevice = kv[1]
		case "rif":
			nif.RemoteInterface = kv[1]
		case "ri":
			nif.RoutingInstance = kv[1]
		case "role":
			nif.Role = kv[1]
		}
	}
}
