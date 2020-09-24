package flowhouse

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/bio-routing/bio-rd/util/grpc/clientmanager"
	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/bio-routing/flowhouse/pkg/frontend"
	"github.com/bio-routing/flowhouse/pkg/intfmapper"
	"github.com/bio-routing/flowhouse/pkg/ipannotator"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/routemirror"
	"github.com/bio-routing/flowhouse/pkg/servers/sflow"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	bnet "github.com/bio-routing/bio-rd/net"
	log "github.com/sirupsen/logrus"
)

// Flowhouse is an clickhouse based sflow collector
type Flowhouse struct {
	cfg               *Config
	ifMapper          *intfmapper.IntfMapper
	routeMirror       *routemirror.RouteMirror
	grpcClientManager *clientmanager.ClientManager
	ipa               *ipannotator.IPAnnotator
	sfs               *sflow.SflowServer
	chgw              *clickhousegw.ClickHouseGateway
	fe                *frontend.Frontend
	flowsRX           chan []*flow.Flow
}

// Config is flow house instances configuration
type Config struct {
	ChCfg         *clickhousegw.ClickhouseConfig
	SNMPCommunity string
	RISTimeout    time.Duration
	ListenSflow   string
	ListenHTTP    string
	DefaultVRF    uint64
	Dicts         frontend.Dicts
}

// ClickhouseConfig represents a clickhouse client config
type ClickhouseConfig struct {
	Host     string
	Address  string
	User     string
	Password string
	Database string
}

// New creates a new flowhouse instance
func New(cfg *Config) (*Flowhouse, error) {
	fh := &Flowhouse{
		cfg:               cfg,
		ifMapper:          intfmapper.New(),
		routeMirror:       routemirror.New(),
		grpcClientManager: clientmanager.New(),
		flowsRX:           make(chan []*flow.Flow, 1024),
	}

	fh.ipa = ipannotator.New(fh.routeMirror)

	sfs, err := sflow.New(fh.cfg.ListenSflow, runtime.NumCPU(), fh.flowsRX, fh.ifMapper)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to start sflow server")
	}
	fh.sfs = sfs

	chgw, err := clickhousegw.New(fh.cfg.ChCfg)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create clickhouse wrapper")
	}
	fh.chgw = chgw

	fh.fe = frontend.New(fh.chgw, cfg.Dicts)
	return fh, nil
}

// AddAgent adds an agent
func (f *Flowhouse) AddAgent(name string, addr bnet.IP, risAddrs []string, vrfs []uint64) {
	f.ifMapper.AddDevice(addr, f.cfg.SNMPCommunity)

	rtSource := make([]*grpc.ClientConn, 0)
	for _, risAddr := range risAddrs {
		f.grpcClientManager.AddIfNotExists(risAddr, grpc.WithInsecure(), grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                f.cfg.RISTimeout,
			Timeout:             f.cfg.RISTimeout,
			PermitWithoutStream: true,
		}))

		rtSource = append(rtSource, f.grpcClientManager.Get(risAddr))
	}

	for _, v := range vrfs {
		fmt.Printf("Adding Target %s %s %v %d\n", name, addr.String(), rtSource, v)
		f.routeMirror.AddTarget(name, addr, rtSource, v)
	}
}

// Run runs flowhouse
func (f *Flowhouse) Run() {
	f.installHTTPHandlers(f.fe)
	go http.ListenAndServe(f.cfg.ListenHTTP, nil)
	log.WithField("address", f.cfg.ListenHTTP).Info("Listening for HTTP requests")

	for {
		flows := <-f.flowsRX

		for _, fl := range flows {
			fl.VRFIn = f.cfg.DefaultVRF
			fl.VRFOut = f.cfg.DefaultVRF

			err := f.ipa.Annotate(fl)
			if err != nil {
				log.WithError(err).Info("Annotating failed")
			}
		}

		err := f.chgw.InsertFlows(flows)
		if err != nil {
			log.WithError(err).Error("Insert failed")
		}
	}
}

func (f *Flowhouse) installHTTPHandlers(fe *frontend.Frontend) {
	http.HandleFunc("/", fe.IndexHandler)
	http.HandleFunc("/flowhouse.js", fe.FlowhouseJSHandler)
	http.HandleFunc("/query", fe.QueryHandler)
	http.HandleFunc("/dict_values/", fe.GetDictValues)
	http.Handle("/metrics", promhttp.Handler())
}
