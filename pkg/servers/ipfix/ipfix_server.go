package ipfix

import (
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/packet/ipfix"
	"github.com/bio-routing/flowhouse/pkg/servers/aggregator"
	"github.com/bio-routing/tflow2/convert"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type InterfaceResolver interface {
	Resolve(agent bnet.IP, ifID uint32) string
}

// fieldMap describes what information is at what index in the slice
// that we get from decoding a netflow packet
type fieldMap struct {
	srcAddr                int
	dstAddr                int
	protocol               int
	packets                int
	size                   int
	intIn                  int
	intOut                 int
	nextHop                int
	family                 int
	vlan                   int
	ts                     int
	srcAsn                 int
	dstAsn                 int
	srcPort                int
	dstPort                int
	samplingPacketInterval int
	srcTos                 int
	srcMask                int
	dstMask                int
	srcMask6               int
	dstMask6               int
	samplingInterval       int
}

type IPFIXServer struct {
	// tmplCache is used to save received flow templates
	// for later lookup in order to decode netflow packets
	tmplCache       *templateCache
	conn            *net.UDPConn
	ifResolver      InterfaceResolver
	output          chan []*flow.Flow
	wg              sync.WaitGroup
	stopCh          chan struct{}
	aggregator      *aggregator.Aggregator
	sampleRateCache *sampleRateCache
}

// New creates and starts a new `IPFIXServer` instance
func New(listen string, numReaders int, output chan []*flow.Flow, ifResolver InterfaceResolver) (*IPFIXServer, error) {
	ipf := &IPFIXServer{
		tmplCache:       newTemplateCache(),
		ifResolver:      ifResolver,
		stopCh:          make(chan struct{}),
		output:          output,
		aggregator:      aggregator.New(output),
		sampleRateCache: newSampleRateCache(),
	}

	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to resolve UDP address")
	}

	con, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, errors.Wrap(err, "ListenUDP failed")
	}
	ipf.conn = con

	ipf.startService(numReaders)
	return ipf, nil
}

func (ipf *IPFIXServer) startService(numReaders int) {
	for i := 0; i < numReaders; i++ {
		ipf.wg.Add(1)
		go func() {
			defer ipf.wg.Done()
			err := ipf.packetWorker()
			if err != nil {
				log.WithError(err).Error("packetWorker failed")
			}
		}()
	}
}

// Stop closes the socket and stops the workers
func (ipf *IPFIXServer) Stop() {
	log.Info("Stopping IPFIX server")
	debug.PrintStack()
	close(ipf.stopCh)
	ipf.conn.Close()
	ipf.aggregator.Stop()
	ipf.wg.Wait()
}

// packetWorker reads sflow packet from socket and handsoff processing to ???
func (ipf *IPFIXServer) packetWorker() error {
	buffer := make([]byte, 8960)
	for {
		if ipf.stopped() {
			return nil
		}

		length, remote, err := ipf.conn.ReadFromUDP(buffer)
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "ReadFromUDP failed")
		}

		remote4 := remote.IP.To4()
		if remote4 != nil {
			remote.IP = remote4
		}

		remoteAddr, err := bnet.IPFromBytes([]byte(remote.IP))
		if err != nil {
			return errors.Wrapf(err, "Unable to convert net.IP to bnet.IP: %q", remote)
		}

		ipf.processPacket(remoteAddr, buffer[:length])
	}
}

func (ipf *IPFIXServer) stopped() bool {
	select {
	case <-ipf.stopCh:
		return true
	default:
		return false
	}
}

func (ipf *IPFIXServer) processPacket(agent bnet.IP, buffer []byte) {
	pkt, err := ipfix.Decode(buffer)
	if err != nil {
		log.WithError(err).Error("Unable to decode IPFIX packet")
		return
	}

	ipf.updateTemplateCache(agent, pkt)
	ipf.processFlowSets(agent, pkt.Header.DomainID, pkt.DataFlowSets(), int64(pkt.Header.ExportTime))
}

// processFlowSets iterates over flowSets and calls processFlowSet() for each flow set
func (ipf *IPFIXServer) processFlowSets(remote bnet.IP, observationDomainID uint32, flowSets []*ipfix.FlowSet, ts int64) {
	addr := remote.String()
	for _, set := range flowSets {
		template, isOpts := ipf.tmplCache.get(remote, observationDomainID, set.Header.SetID)

		if template == nil {
			templateKey := makeTemplateKey(addr, observationDomainID, set.Header.SetID)
			log.Debugf("Template for given FlowSet not found: %s", templateKey)

			continue
		}

		records := ipfix.DecodeFlowSet(*set, template)
		if records == nil {
			log.Warning("Error decoding FlowSet")
			continue
		}

		ipf.processFlowSet(template, records, remote, observationDomainID, ts, isOpts)
	}
}

// process generates Flow elements from records and pushes them into the `receiver` channel
func (ipf *IPFIXServer) processFlowSet(template []*ipfix.TemplateRecord, records []ipfix.FlowDataRecord, agent bnet.IP, observationDomainID uint32, ts int64, isOpts bool) {
	fm := generateFieldMap(template)

	for _, r := range records {
		if isOpts {
			if fm.samplingInterval > 0 {
				sampleRate := convert.Uint32(r.Values[fm.samplingInterval])
				ipf.sampleRateCache.set(agent, observationDomainID, sampleRate)
			}

			continue
		}

		fl := &flow.Flow{
			Agent:     agent,
			Timestamp: ts,
		}

		if fm.family >= 0 {
			fl.Family = uint8(fm.family)
		}

		if fm.packets >= 0 {
			fl.Packets = convert.Uint64(r.Values[fm.packets])
		}

		if fm.size >= 0 {
			fl.Size = uint64(convert.Uint32(r.Values[fm.size]))
		}

		if fm.protocol >= 0 {
			fl.Protocol = uint8(convert.Uint16(r.Values[fm.protocol]))
		}

		if fm.intIn >= 0 {
			fl.IntIn = ipf.ifResolver.Resolve(agent, convert.Uint32(r.Values[fm.intIn]))
		}

		if fm.intOut >= 0 {
			fl.IntOut = ipf.ifResolver.Resolve(agent, convert.Uint32(r.Values[fm.intOut]))
		}

		if fm.srcPort >= 0 {
			fl.SrcPort = convert.Uint16(r.Values[fm.srcPort])
		}

		if fm.dstPort >= 0 {
			fl.DstPort = convert.Uint16(r.Values[fm.dstPort])
		}

		if fm.srcAddr >= 0 {
			fl.SrcAddr = bnet.IPv4FromBytes(convert.Reverse(r.Values[fm.srcAddr]))
		}

		if fm.dstAddr >= 0 {
			fl.DstAddr = bnet.IPv4FromBytes(convert.Reverse(r.Values[fm.dstAddr]))
		}

		if fm.nextHop >= 0 {
			fl.NextHop = bnet.IPv4FromBytes(convert.Reverse(r.Values[fm.nextHop]))
		}

		if fm.srcTos >= 0 {
			fl.TOS = uint8(r.Values[fm.srcTos][0])
		}

		if fm.dstAsn >= 0 {
			fl.DstAs = convert.Uint32(r.Values[fm.dstAsn])
		}

		if fm.srcAsn >= 0 {
			fl.SrcAs = convert.Uint32(r.Values[fm.srcAsn])
		}

		if fm.srcMask > 0 {
			mask := uint8(r.Values[fm.srcMask][0])
			p := bnet.NewPfx(fl.SrcAddr, mask)
			p.BaseAddr()
			fl.SrcPfx = bnet.NewPfx(*p.BaseAddr(), mask)
		}

		if fm.dstMask > 0 {
			mask := uint8(r.Values[fm.dstMask][0])
			p := bnet.NewPfx(fl.DstAddr, mask)
			p.BaseAddr()
			fl.DstPfx = bnet.NewPfx(*p.BaseAddr(), mask)
		}

		if fm.srcMask6 > 0 {
			mask := uint8(r.Values[fm.srcMask6][0])
			p := bnet.NewPfx(fl.SrcAddr, mask)
			p.BaseAddr()
			fl.SrcPfx = bnet.NewPfx(*p.BaseAddr(), mask)
		}

		if fm.dstMask6 > 0 {
			mask := uint8(r.Values[fm.dstMask6][0])
			p := bnet.NewPfx(fl.DstAddr, mask)
			p.BaseAddr()
			fl.DstPfx = bnet.NewPfx(*p.BaseAddr(), mask)
		}

		fl.Samplerate = uint64(ipf.sampleRateCache.get(agent, observationDomainID))

		ipf.aggregator.GetIngress() <- fl
	}
}

// generateFieldMap processes a TemplateRecord and populates a fieldMap accordingly
// the FieldMap can then be used to read fields from a flow
func generateFieldMap(template []*ipfix.TemplateRecord) *fieldMap {
	fm := fieldMap{
		srcAddr:                -1,
		dstAddr:                -1,
		protocol:               -1,
		packets:                -1,
		size:                   -1,
		intIn:                  -1,
		intOut:                 -1,
		nextHop:                -1,
		family:                 -1,
		vlan:                   -1,
		ts:                     -1,
		srcAsn:                 -1,
		dstAsn:                 -1,
		srcPort:                -1,
		dstPort:                -1,
		samplingPacketInterval: -1,
		srcTos:                 -1,
		srcMask:                -1,
		dstMask:                -1,
		srcMask6:               -1,
		dstMask6:               -1,
		samplingInterval:       -1,
	}

	i := -1
	for _, f := range template {
		i++

		switch f.Type {
		case ipfix.IPv4SrcAddr:
			fm.srcAddr = i
			fm.family = 4
		case ipfix.IPv6SrcAddr:
			fm.srcAddr = i
			fm.family = 6
		case ipfix.IPv4DstAddr:
			fm.dstAddr = i
		case ipfix.IPv6DstAddr:
			fm.dstAddr = i
		case ipfix.InBytes:
			fm.size = i
		case ipfix.Protocol:
			fm.protocol = i
		case ipfix.InPkts:
			fm.packets = i
		case ipfix.InputSnmp:
			fm.intIn = i
		case ipfix.OutputSnmp:
			fm.intOut = i
		case ipfix.IPv4NextHop:
			fm.nextHop = i
		case ipfix.IPv6NextHop:
			fm.nextHop = i
		case ipfix.L4SrcPort:
			fm.srcPort = i
		case ipfix.L4DstPort:
			fm.dstPort = i
		case ipfix.SrcAs:
			fm.srcAsn = i
		case ipfix.DstAs:
			fm.dstAsn = i
		case ipfix.SamplingPacketInterval:
			fm.samplingPacketInterval = i
		case ipfix.SrcTos:
			fm.srcTos = i
		case ipfix.SrcMask:
			fm.srcMask = i
		case ipfix.DstMask:
			fm.dstMask = i
		case ipfix.IPv6SrcMask:
			fm.srcMask6 = i
		case ipfix.IPv6DstMask:
			fm.dstMask6 = i
		case ipfix.SamplingInterval:
			fm.samplingInterval = i
		}
	}

	return &fm
}

// updateTemplateCache updates the template cache
func (ipf *IPFIXServer) updateTemplateCache(remote bnet.IP, p *ipfix.Packet) {
	templRecs := p.GetTemplateRecords()
	for _, tr := range templRecs {
		ipf.tmplCache.set(remote, p.Header.DomainID, tr.Header.TemplateID, tr.Records, false)
	}

	optTemplRecs := p.GetOptionTemplateRecords()
	for _, tr := range optTemplRecs {
		ipf.tmplCache.set(remote, p.Header.DomainID, tr.Header.TemplateID, tr.Records, true)
	}
}

// makeTemplateKey creates a string of the 3 tuple router address, source id and template id
func makeTemplateKey(addr string, sourceID uint32, templateID uint16) string {
	keyParts := []string{
		addr,
		strconv.Itoa(int(sourceID)),
		strconv.Itoa(int(templateID)),
	}
	return strings.Join(keyParts, "|")
}
