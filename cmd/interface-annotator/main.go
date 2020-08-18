package main

import (
	"flag"
	"time"

	"github.com/bio-routing/flowhouse/pkg/ifnamecollector"
	"github.com/bio-routing/flowhouse/pkg/models/ifnames"

	bnet "github.com/bio-routing/bio-rd/net"
	log "github.com/sirupsen/logrus"
)

var (
	community  = flag.String("community", "public", "SNMP community")
	target     = flag.String("target", "", "Target device")
	dbHost     = flag.String("db-host", "localhost", "Clickhouse host")
	dbPort     = flag.Uint("db-port", 9000, "Clickhouse port")
	dbUser     = flag.String("db-user", "default", "Clickhouse user")
	dbPassword = flag.String("db-password", "", "Clickhouse password")
)

func main() {
	flag.Parse()

	if *target == "" {
		log.Panic("No target given")
	}

	t, err := bnet.IPFromString(*target)
	if err != nil {
		log.WithError(err).Panic("Unable to parse target IP address")
	}

	h, err := ifnames.New(*dbHost, uint16(*dbPort), *dbUser, *dbPassword)
	if err != nil {
		log.WithError(err).Panic("Unable to get interface names client")
	}

	c := ifnamecollector.NewIfNameCollector(t.String(), *community)

	for {
		collect(h, c, t)
		time.Sleep(time.Minute)
	}
}

func collect(h *ifnames.IfNamesSaver, c *ifnamecollector.IfNameCollector, t bnet.IP) {
	mapping, err := c.Collect()
	if err != nil {
		log.WithError(err).Errorf("Collect failed")
		return
	}

	err = h.Insert(t, mapping)
	if err != nil {
		log.WithError(err).Errorf("Insert failed")
		return
	}
}
