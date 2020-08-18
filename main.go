package main

import (
	"flag"
	"fmt"
	"runtime"

	"net/http"

	"github.com/bio-routing/flowhouse/pkg/clickhousegw"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/servers/sflow"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	log "github.com/sirupsen/logrus"
)

var (
	listenSflow = flag.String("sflow-listen", ":6343", "Listening UDP address for sflow packets")
	listenHTTP  = flag.String("http-listen", ":9991", "Listening address for HTTP service")
)

func main() {
	flag.Parse()

	chg, err := clickhousegw.New()
	if err != nil {
		log.WithError(err).Panic("Unable to create clickhouse gateway")
	}
	_ = chg

	outCh := make(chan []*flow.Flow, 1024)
	sfs, err := sflow.New(*listenSflow, runtime.NumCPU(), outCh)
	if err != nil {
		log.WithError(err).Panic("Unable to start sflow server")
	}
	defer sfs.Stop()

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*listenHTTP, nil)

	for {
		flows := <-outCh
		fmt.Printf("Flows: %d\n", len(flows))
		err := chg.Insert(flows)
		if err != nil {
			log.WithError(err).Error("Insert failed")
		}
	}

	select {}
}
