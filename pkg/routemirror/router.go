package routemirror

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
)

type router struct {
	name    string
	address net.IP
	sources []*grpc.ClientConn
	vrfs    map[uint64]*routerVRF
}

func newRouter(name string, address net.IP, sources []*grpc.ClientConn) *router {
	return &router{
		name:    name,
		address: address,
		sources: sources,
		vrfs:    make(map[uint64]*routerVRF),
	}
}

func (r *router) addVRFIfNotExists(rd uint64) *routerVRF {
	if _, exists := r.vrfs[rd]; !exists {
		r.vrfs[rd] = newRouterVRF(r, rd)
		for _, s := range r.sources {
			r.vrfs[rd].addRIS(s)
		}
	}

	return r.vrfs[rd]
}

func (r *router) getVRF(rd uint64) *routerVRF {
	if _, exists := r.vrfs[rd]; !exists {
		return nil
	}

	return r.vrfs[rd]
}

func (r *router) removeVRF(rd uint64) error {
	if _, exists := r.vrfs[rd]; !exists {
		return fmt.Errorf("VRF %d not found", rd)
	}

	// TODO: Implement
	return nil
}

func (r *router) stop() {
	for _, v := range r.vrfs {
		v.stop()
	}
}
