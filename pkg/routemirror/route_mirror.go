package routemirror

import (
	"fmt"
	"net"
	"sync"

	"github.com/bio-routing/bio-rd/route"
	"google.golang.org/grpc"

	bnet "github.com/bio-routing/bio-rd/net"
)

// RouteMirror is a RIS based route mirror
type RouteMirror struct {
	routers   map[string]*router
	routersMu sync.RWMutex
}

// New creates a new RouteMirror
func New() *RouteMirror {
	return &RouteMirror{
		routers: make(map[string]*router),
	}
}

// AddTarget adds a target
func (r *RouteMirror) AddTarget(name string, address net.IP, sources []*grpc.ClientConn, vrfRD uint64) {
	r.routersMu.Lock()
	defer r.routersMu.Unlock()

	rtr := r.addRouterIfNotExists(name, address, sources)
	rtr.addVRFIfNotExists(vrfRD)
}

func (r *RouteMirror) addRouterIfNotExists(name string, address net.IP, sources []*grpc.ClientConn) *router {
	if _, exists := r.routers[name]; exists {
		return r.routers[name]
	}

	rtr := newRouter(name, address, sources)
	r.routers[name] = rtr

	return rtr
}

// Stop stops the route mirror
func (r *RouteMirror) Stop() {
	r.routersMu.Lock()
	defer r.routersMu.Unlock()

	for _, rtr := range r.routers {
		rtr.stop()
	}
}

func (r *RouteMirror) getRouter(needle string) *router {
	r.routersMu.RLock()
	defer r.routersMu.RUnlock()

	for _, rtr := range r.routers {
		if rtr.address.String() == needle {
			return rtr
		}
	}

	return nil
}

// LPM preforms a Longest Prefix Match against a routers VRF
func (r *RouteMirror) LPM(rtrAddr string, vrfRD uint64, addr bnet.IP) (*route.Route, error) {
	rtr := r.getRouter(rtrAddr)
	if rtr == nil {
		return nil, fmt.Errorf("Router not found")
	}

	afi := uint8(6)
	pfxLen := uint8(128)
	if addr.IsIPv4() {
		afi = 4
		pfxLen = 32
	}

	v := rtr.getVRF(vrfRD)
	if v == nil {
		return nil, fmt.Errorf("Invalid VRF %d pn %q", vrfRD, rtrAddr)
	}

	rib := v.getLocRIB(afi)
	routes := rib.LPM(bnet.NewPfx(addr, pfxLen).Ptr())

	if len(routes) == 0 {
		return nil, nil
	}

	// TODO: Check if PathSelection() in bio is wrong. Best route is apparently last element. Should be first...
	return routes[len(routes)-1], nil
}
