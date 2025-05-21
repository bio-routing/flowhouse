package flowhouse

import (
	"net/http"
	"runtime"
	"time"
	"fmt"
	"runtime/debug"

	"github.com/bio-routing/bio-rd/util/grpc/clientmanager"
	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/bio-routing/flowhouse/pkg/frontend"
	"github.com/bio-routing/flowhouse/pkg/intfmapper"
	"github.com/bio-routing/flowhouse/pkg/ipannotator"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/routemirror"
	"github.com/bio-routing/flowhouse/pkg/servers/ipfix"
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
	ifxs              *ipfix.IPFIXServer
	chgw              *clickhousegw.ClickHouseGateway
	fe                *frontend.Frontend
	flowsRX           chan []*flow.Flow
}

// Config is flow house instances configuration
type Config struct {
	ChCfg              *clickhousegw.ClickhouseConfig
	SNMP               *config.SNMPConfig
	RISTimeout         time.Duration
	ListenSflow        string
	ListenIPFIX        string
	ListenHTTP         string
	DefaultVRF         uint64
	Dicts              frontend.Dicts
	DisableIPAnnotator bool
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

	if !cfg.DisableIPAnnotator {
		fh.ipa = ipannotator.New(fh.routeMirror)
	}

	sfs, err := sflow.New(fh.cfg.ListenSflow, runtime.NumCPU(), fh.flowsRX, fh.ifMapper)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to start sflow server")
	}
	fh.sfs = sfs

	ifxs, err := ipfix.New(fh.cfg.ListenIPFIX, runtime.NumCPU(), fh.flowsRX, fh.ifMapper)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to start IPFIX server")
	}
	fh.ifxs = ifxs

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
	if f.cfg.SNMP != nil {
		f.ifMapper.AddDevice(addr, f.cfg.SNMP)
	}

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

		if f.ipa != nil {
			for _, fl := range flows {
				fl.VRFIn = f.cfg.DefaultVRF
				fl.VRFOut = f.cfg.DefaultVRF

				err := f.ipa.Annotate(fl)
				if err != nil {
					log.WithError(err).Info("Annotating failed")
				}
			}
		}

		err := f.chgw.InsertFlows(flows)
		if err != nil {
			log.WithError(err).Error("Insert failed")
		}
	}
}

func recoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("PANIC: %v\n%s", err, debug.Stack())
                http.Error(w,
                    fmt.Sprintf("Internal server error: %v", err),
                    http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}

func (f *Flowhouse) installHTTPHandlers(fe *frontend.Frontend) {
    http.HandleFunc("/", fe.IndexHandler)
    http.HandleFunc("/flowhouse.js", fe.FlowhouseJSHandler)
    http.Handle("/query", recoveryMiddleware(http.HandlerFunc(fe.QueryHandler)))
    http.Handle("/dict_values/", recoveryMiddleware(http.HandlerFunc(fe.GetDictValues)))
    http.Handle("/metrics", promhttp.Handler())
}
