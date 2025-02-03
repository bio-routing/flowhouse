package main

import (
	"flag"
	"sync"
	"time"

	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/flowhouse"

	log "github.com/sirupsen/logrus"
)

var (
	configFilePath = flag.String("config.file", "config.yaml", "Config file path (YAML)")
	debug          = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("logLevel: DEBUG")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	cfg, err := config.GetConfig(*configFilePath)
	if err != nil {
		log.WithError(err).Fatal("Unable to get config")
	}

	fhcfg := &flowhouse.Config{
		ChCfg:              cfg.Clickhouse,
		SNMP:               cfg.SNMP,
		RISTimeout:         time.Duration(cfg.RISTimeout) * time.Second,
		ListenSflow:        cfg.ListenSFlow,
		ListenIPFIX:        cfg.ListenIPFIX,
		ListenHTTP:         cfg.ListenHTTP,
		DefaultVRF:         cfg.GetDefaultVRF(),
		Dicts:              cfg.Dicts,
		DisableIPAnnotator: cfg.DisableIPAnnotator,
	}

	fh, err := flowhouse.New(fhcfg)
	if err != nil {
		log.WithError(err).Fatal("Unable to create flowhouse instance")
	}

	for _, rtr := range cfg.Routers {
		fh.AddAgent(rtr.Name, rtr.GetAddress(), rtr.RISInstances, rtr.GetVRFs())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fh.Run()
	}()

	wg.Wait()
}
