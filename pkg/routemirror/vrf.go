package routemirror

import (
	"github.com/bio-routing/bio-rd/cmd/ris/api"
	"github.com/bio-routing/bio-rd/risclient"
	"github.com/bio-routing/bio-rd/routingtable/locRIB"
	"github.com/bio-routing/bio-rd/routingtable/mergedlocrib"
	"google.golang.org/grpc"
)

type routerVRF struct {
	router           *router
	rd               uint64
	locRIBIPv4       *locRIB.LocRIB
	locRIBIPv6       *locRIB.LocRIB
	mergedLocRIBIPv4 *mergedlocrib.MergedLocRIB
	mergedLocRIBIPv6 *mergedlocrib.MergedLocRIB
	risClients       []*risclient.RISClient
}

func newRouterVRF(router *router, vrfRD uint64) *routerVRF {
	v := &routerVRF{
		router:     router,
		rd:         vrfRD,
		locRIBIPv4: locRIB.New("inet.0"),
		locRIBIPv6: locRIB.New("inet6.0"),
		risClients: make([]*risclient.RISClient, 0),
	}

	v.mergedLocRIBIPv4 = mergedlocrib.New(v.locRIBIPv4)
	v.mergedLocRIBIPv6 = mergedlocrib.New(v.locRIBIPv6)

	return v
}

func (v *routerVRF) stop() {
	for _, rc := range v.risClients {
		rc.Stop()
	}
}

func (v *routerVRF) addRIS(cc *grpc.ClientConn) {
	for _, afi := range []uint8{0, 1} {
		c := v.mergedLocRIBIPv4
		if afi == 1 {
			c = v.mergedLocRIBIPv6
		}

		rc := risclient.New(&risclient.Request{
			Router: v.router.address.String(),
			VRFRD:  v.rd,
			AFI:    api.ObserveRIBRequest_AFISAFI(afi),
		}, cc, c)

		v.risClients = append(v.risClients, rc)
		rc.Start()
	}
}

func (v *routerVRF) getLocRIB(afi uint8) *locRIB.LocRIB {
	if afi == 6 {
		return v.locRIBIPv6
	}

	return v.locRIBIPv4
}
