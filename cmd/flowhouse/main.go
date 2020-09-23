package main

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/bio-routing/bio-rd/routingtable/vrf"
	"github.com/bio-routing/bio-rd/util/grpc/clientmanager"
	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/bio-routing/flowhouse/pkg/frontend"
	"github.com/bio-routing/flowhouse/pkg/intfmapper"
	"github.com/bio-routing/flowhouse/pkg/ipannotator"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/routemirror"
	"github.com/bio-routing/flowhouse/pkg/servers/sflow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	log "github.com/sirupsen/logrus"
)

var (
	configFilePath = flag.String("config.file", "config.yaml", "Config file path (YAML)")
)

func main() {
	flag.Parse()

	fmt.Printf("Getting config\n")
	cfg, err := config.GetConfig(*configFilePath)
	if err != nil {
		log.WithError(err).Fatal("Unable to get config")
	}
	_ = cfg

	fmt.Printf("Setting up client manager\n")
	cm := setupClientManager(cfg)
	// TODO: Add cm.Stop() function

	_ = cm
	fmt.Printf("Setting up route mirror\n")
	rm := setupRouteMirror(cfg, cm)
	//defer rm.Stop()

	ipa := ipannotator.New(rm)

	if cfg.Clickhouse == nil {
		log.Fatalf("Clickhouse config is missing")
	}

	chg, err := clickhousegw.New(cfg.Clickhouse)
	if err != nil {
		log.WithError(err).Fatal("Unable to create clickhouse gateway")
	}
	defer chg.Close()

	ifMapper := intfmapper.New()
	for _, rtr := range cfg.Routers {
		ifMapper.AddDevice(rtr.GetAddress(), cfg.SNMPCommunity)
	}

	outCh := make(chan []*flow.Flow, 1024)
	sfs, err := sflow.New(cfg.ListenSFlow, runtime.NumCPU(), outCh, ifMapper)
	if err != nil {
		log.WithError(err).Panic("Unable to start sflow server")
	}
	//defer sfs.Stop()
	_ = sfs

	fe := frontend.New(chg, cfg.Dicts)
	http.HandleFunc("/", fe.IndexHandler)
	http.HandleFunc("/flowhouse.js", fe.FlowhouseJSHandler)
	http.HandleFunc("/query", fe.QueryHandler)
	http.HandleFunc("/dict_values/", fe.GetDictValues)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(cfg.ListenHTTP, nil)
	log.WithField("address", cfg.ListenHTTP).Info("Listening for HTTP requests")

	vrfID, _ := vrf.ParseHumanReadableRouteDistinguisher("51324:65201")

	for {
		flows := <-outCh
		fmt.Printf("Flows: %d\n", len(flows))

		for _, fl := range flows {
			fl.VRFIn = vrfID
			fl.VRFOut = vrfID

			err := ipa.Annotate(fl)
			if err != nil {
				log.WithError(err).Error("Annotating failed")
			}
		}

		err := chg.InsertFlows(flows)
		if err != nil {
			log.WithError(err).Error("Insert failed")
		}
	}
}

func setupClientManager(cfg *config.Config) *clientmanager.ClientManager {
	cm := clientmanager.New()
	for _, ris := range cfg.GetRISList() {
		err := cm.Add(ris, grpc.WithInsecure(), grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Second * time.Duration(cfg.RISTimeout),
			Timeout:             time.Second * time.Duration(cfg.RISTimeout),
			PermitWithoutStream: true,
		}))

		if err != nil {
			log.WithError(err).Fatal("Unable to add RIS instance")
		}
	}

	return cm
}

func setupRouteMirror(cfg *config.Config, cm *clientmanager.ClientManager) *routemirror.RouteMirror {
	ra := routemirror.New()
	for _, rtr := range cfg.Routers {
		sources := make([]*grpc.ClientConn, 0)
		for _, risInstanceAddr := range rtr.RISInstances {
			cc := cm.Get(risInstanceAddr)
			if cc == nil {
				log.Fatalf("Unable to find grpc client for %q", risInstanceAddr)
			}

			sources = append(sources, cc)
		}

		for _, v := range rtr.GetVRFs() {
			ra.AddTarget(rtr.Name, rtr.GetAddress(), sources, v)
		}
	}

	return ra
}
