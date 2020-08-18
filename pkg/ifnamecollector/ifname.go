package ifnamecollector

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	g "github.com/soniah/gosnmp"
)

const (
	ifNameOID  = "1.3.6.1.2.1.31.1.1.1.1"
	ifAliasOID = "1.3.6.1.2.1.31.1.1.1.18"
	timeout    = time.Second * 30
)

// IfNameCollector collects interface names from devices via SNMP
type IfNameCollector struct {
	device    string
	community string
}

// InterfaceIDByName maps interface names to IDs
type InterfaceIDByName map[uint32]*netInterface

func NewIfNameCollector(device string, community string) *IfNameCollector {
	return &IfNameCollector{
		device:    device,
		community: community,
	}
}

func (inc *IfNameCollector) Collect() (InterfaceIDByName, error) {
	var snmpClient *g.GoSNMP
	tmp := *g.Default
	snmpClient = &tmp
	snmpClient.Target = inc.device
	snmpClient.Community = inc.community
	snmpClient.Timeout = timeout

	if err := snmpClient.Connect(); err != nil {
		return nil, errors.Wrap(err, "SNMP client unable to connect")
	}
	defer snmpClient.Conn.Close()

	interfaces := make(InterfaceIDByName)
	err := snmpClient.BulkWalk(ifAliasOID, interfaces.updateIfAlias)
	if err != nil {
		return nil, errors.Wrap(err, "walk error")
	}

	err = snmpClient.BulkWalk(ifNameOID, interfaces.updateIfNames)
	if err != nil {
		return nil, errors.Wrap(err, "walk error")
	}

	return interfaces, nil
}

func (im InterfaceIDByName) updateIfAlias(pdu g.SnmpPDU) error {
	oid := strings.Split(pdu.Name, ".")
	id, err := strconv.Atoi(oid[len(oid)-1])
	if err != nil {
		return errors.Wrap(err, "Unable to convert interface id")
	}

	if pdu.Type != g.OctetString {
		return errors.Errorf("Unexpected PDU type: %d", pdu.Type)
	}

	netif := newNetInterface(uint32(id), string(pdu.Value.([]byte)))
	im[netif.ID] = netif
	return nil
}

func (im InterfaceIDByName) updateIfNames(pdu g.SnmpPDU) error {
	oid := strings.Split(pdu.Name, ".")
	id, err := strconv.Atoi(oid[len(oid)-1])
	if err != nil {
		return errors.Wrap(err, "Unable to convert interface id")
	}

	if pdu.Type != g.OctetString {
		return errors.Errorf("Unexpected PDU type: %d", pdu.Type)
	}

	if _, exists := im[uint32(id)]; !exists {
		return nil
	}

	im[uint32(id)].Name = string(pdu.Value.([]byte))
	return nil
}
