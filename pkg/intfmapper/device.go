package intfmapper

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/gosnmp/gosnmp"
	"github.com/pkg/errors"

	bnet "github.com/bio-routing/bio-rd/net"
	log "github.com/sirupsen/logrus"
)

const (
	ifNameOID = "1.3.6.1.2.1.31.1.1.1.1"
	snmpPort  = 161
	timeout   = time.Second * 30
)

type device struct {
	addr             bnet.IP
	snmpCfg          *config.SNMPConfig
	interfacesByID   map[uint32]*netIf
	interfacesByName map[string]*netIf
	interfacesMu     sync.RWMutex
	stopCh           chan struct{}
	wg               sync.WaitGroup
	ticker           *time.Ticker
}

func newDevice(addr bnet.IP, snmpCfg *config.SNMPConfig) *device {
	d := &device{
		addr:             addr,
		snmpCfg:          snmpCfg,
		interfacesByID:   make(map[uint32]*netIf),
		interfacesByName: make(map[string]*netIf),
		ticker:           time.NewTicker(time.Minute * 2),
	}

	d.startCollector()
	return d
}

func (d *device) update(interfaces []*netIf) {
	interfacesByID := make(map[uint32]*netIf)
	interfacesByName := make(map[string]*netIf)
	for _, ifa := range interfaces {
		interfacesByID[ifa.id] = ifa
		interfacesByName[ifa.name] = ifa
	}

	d.interfacesMu.Lock()
	defer d.interfacesMu.Unlock()

	d.interfacesByID = interfacesByID
	d.interfacesByName = interfacesByName
}

type netIf struct {
	id   uint32
	name string
}

func (d *device) startCollector() {
	d.wg.Add(1)
	go d.collector()
}

func (d *device) collector() {
	defer d.wg.Done()

	for {
		err := d.collect()
		if err != nil {
			log.WithError(err).Warning("Collecting failed")
			continue
		}

		select {
		case <-d.stopCh:
			return
		case <-d.ticker.C:
		}
	}
}

func (d *device) collect() error {
	s := &gosnmp.GoSNMP{
		Target:                  d.addr.String(),
		Port:                    snmpPort,
		Community:               d.snmpCfg.Community,
		Version:                 gosnmp.Version2c,
		Timeout:                 timeout,
		Retries:                 0,
		ExponentialTimeout:      false,
		UseUnconnectedUDPSocket: true,
	}

	if d.snmpCfg.Version == 3 {
		s.Community = ""
		s.Version = gosnmp.Version3
		s.SecurityModel = gosnmp.UserSecurityModel
		s.MsgFlags = gosnmp.AuthPriv
		s.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 d.snmpCfg.User,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: d.snmpCfg.AuthPassphrase,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        d.snmpCfg.PrivacyPassphrase,
		}
	}

	err := s.Connect()
	if err != nil {
		return errors.Wrap(err, "Unable to connect")
	}

	defer s.Conn.Close()

	interfaces := make([]*netIf, 0)
	err = s.BulkWalk(ifNameOID, func(pdu gosnmp.SnmpPDU) error {
		oid := strings.Split(pdu.Name, ".")
		id, err := strconv.Atoi(oid[len(oid)-1])
		if err != nil {
			return errors.Wrap(err, "Unable to convert interface id")
		}

		if pdu.Type != gosnmp.OctetString {
			return errors.Errorf("Unexpected PDU type: %d", pdu.Type)
		}

		name := string(pdu.Value.([]byte))
		interfaces = append(interfaces, &netIf{
			id:   uint32(id),
			name: name,
		})

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "BulkWalk failed for "+d.addr.String())
	}

	d.update(interfaces)
	return nil
}

func (d *device) resolve(ifID uint32) string {
	d.interfacesMu.RLock()
	defer d.interfacesMu.RUnlock()

	if _, exists := d.interfacesByID[ifID]; !exists {
		return ""
	}

	return d.interfacesByID[ifID].name
}
