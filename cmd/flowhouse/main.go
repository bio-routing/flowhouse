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
)

func main() {
	flag.Parse()

	cfg, err := config.GetConfig(*configFilePath)
	if err != nil {
		log.WithError(err).Fatal("Unable to get config")
	}

	fhcfg := &flowhouse.Config{
		ChCfg:         cfg.Clickhouse,
		SNMPCommunity: cfg.SNMPCommunity,
		RISTimeout:    time.Duration(cfg.RISTimeout) * time.Second,
		ListenSflow:   cfg.ListenSFlow,
		ListenHTTP:    cfg.ListenHTTP,
		DefaultVRF:    cfg.GetDefaultVRF(),
		Dicts:         cfg.Dicts,
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
