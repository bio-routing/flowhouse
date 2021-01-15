// Package sfserver provides sflow collection services via UDP and passes flows into aggregator layer
package sflow

import (
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"

	"github.com/bio-routing/tflow2/convert"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/bio-routing/flowhouse/pkg/packet/packet"
	"github.com/bio-routing/flowhouse/pkg/packet/sflow"
	log "github.com/sirupsen/logrus"
)

var labels []string

func init() {
	labels = []string{
		"agent",
	}
}

type InterfaceResolver interface {
	Resolve(agent bnet.IP, ifID uint32) string
}

// SflowServer represents a sflow Collector instance
type SflowServer struct {
	aggregator               *aggregator
	conn                     *net.UDPConn
	ifResolver               InterfaceResolver
	wg                       sync.WaitGroup
	stopCh                   chan struct{}
	packetsReceived          *prometheus.CounterVec
	flowSamplesReceived      *prometheus.CounterVec
	flowNoRawPktHeader       *prometheus.CounterVec
	flowNoData               *prometheus.CounterVec
	flowUnknownProtocol      *prometheus.CounterVec
	flowEthernetDecodeErrors *prometheus.CounterVec
	flowUnknownEtherType     *prometheus.CounterVec
	flowDot1qDecodeErrors    *prometheus.CounterVec
	flowIPv4DecodeErrors     *prometheus.CounterVec
	flowIPv6DecodeErrors     *prometheus.CounterVec
	flowTCPDecodeErros       *prometheus.CounterVec
	flowUDPDecodeErros       *prometheus.CounterVec
}

// New creates and starts a new `SflowServer` instance
func New(listen string, numReaders int, output chan []*flow.Flow, ifResolver InterfaceResolver) (*SflowServer, error) {
	sfs := &SflowServer{
		aggregator: newAggregator(output),
		ifResolver: ifResolver,
		packetsReceived: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "received_packets",
			Help:      "Received sflow packets",
		}, labels),
		flowSamplesReceived: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_received",
			Help:      "Flow samples received",
		}, labels),
		flowNoRawPktHeader: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_no_raw_pkt_header",
			Help:      "Flow samples without raw packet header",
		}, labels),
		flowNoData: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_no_data",
			Help:      "Flow samples without data",
		}, labels),
		flowUnknownProtocol: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_unknown_protocol",
			Help:      "Flow samples unknown protocol",
		}, labels),
		flowEthernetDecodeErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_ethernet_decode_errors",
			Help:      "Flow samples ethernet decode errors",
		}, labels),
		flowUnknownEtherType: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_unknown_ether_type",
			Help:      "Flow samples unknown ether type",
		}, labels),
		flowDot1qDecodeErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_dot1q_decode_errors",
			Help:      "Flow samples Dot1Q decode errors",
		}, labels),
		flowIPv4DecodeErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_ipv4_decode_errors",
			Help:      "Flow samples IPv4 decode errors",
		}, labels),
		flowIPv6DecodeErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_ipv6_decode_errors",
			Help:      "Flow samples IPv6 decode errors",
		}, labels),
		flowTCPDecodeErros: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_tcp_decode_errors",
			Help:      "Flow samples TCP decode errors",
		}, labels),
		flowUDPDecodeErros: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "flowhouse",
			Subsystem: "sflow",
			Name:      "flow_samples_udp_decode_errors",
			Help:      "Flow samples UDP decode errors",
		}, labels),
		stopCh: make(chan struct{}),
	}

	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to resolve UDP address")
	}

	con, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, errors.Wrap(err, "ListenUDP failed")
	}
	sfs.conn = con

	sfs.startService(numReaders)
	return sfs, nil
}

func (sfs *SflowServer) startService(numReaders int) {
	for i := 0; i < numReaders; i++ {
		sfs.wg.Add(1)
		go func() {
			defer sfs.wg.Done()
			err := sfs.packetWorker()
			if err != nil {
				log.WithError(err).Error("packetWorker failed")
			}
		}()
	}
}

// Stop closes the socket and stops the workers
func (sfs *SflowServer) Stop() {
	log.Info("Stopping SflowServer")
	debug.PrintStack()
	close(sfs.stopCh)
	sfs.aggregator.stop()
	sfs.conn.Close()
	sfs.wg.Wait()
}

// packetWorker reads sflow packet from socket and handsoff processing to ???
func (sfs *SflowServer) packetWorker() error {
	buffer := make([]byte, 8960)
	for {
		if sfs.stopped() {
			return nil
		}

		length, remote, err := sfs.conn.ReadFromUDP(buffer)
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

		sfs.packetsReceived.WithLabelValues(remoteAddr.String()).Inc()
		sfs.processPacket(remoteAddr, buffer[:length])
	}
}

func (sfs *SflowServer) stopped() bool {
	select {
	case <-sfs.stopCh:
		return true
	default:
		return false
	}
}

// processPacket takes a raw sflow packet, send it to the decoder and passes the decoded packet to the aggregator
func (sfs *SflowServer) processPacket(agent bnet.IP, buffer []byte) {
	agentStr := agent.String()

	p, err := sflow.Decode(buffer[:len(buffer)])
	if err != nil {
		log.WithError(err).Error("Unable to decode sflow packet")
		return
	}

	for _, fs := range p.FlowSamples {
		sfs.flowSamplesReceived.WithLabelValues(agentStr).Inc()

		if fs.RawPacketHeader == nil {
			sfs.flowNoRawPktHeader.WithLabelValues(agentStr).Inc()
			continue
		}

		if fs.Data == nil {
			sfs.flowNoData.WithLabelValues(agentStr).Inc()
			continue
		}

		if fs.RawPacketHeader.HeaderProtocol != 1 {
			sfs.flowUnknownProtocol.WithLabelValues(agentStr).Inc()
			continue
		}

		ether, err := packet.DecodeEthernet(fs.Data, fs.RawPacketHeader.OriginalPacketLength)
		if err != nil {
			sfs.flowEthernetDecodeErrors.WithLabelValues(agentStr).Inc()
			log.WithError(err).Debug("Unable to decode ethernet packet")
			continue
		}
		fs.Data = unsafe.Pointer(uintptr(fs.Data) - packet.SizeOfEthernetII)
		fs.DataLen -= uint32(packet.SizeOfEthernetII)

		fl := &flow.Flow{
			Agent:      agent,
			IntIn:      sfs.ifResolver.Resolve(agent, fs.FlowSampleHeader.InputIf),
			IntOut:     sfs.ifResolver.Resolve(agent, fs.FlowSampleHeader.OutputIf),
			Size:       uint64(fs.RawPacketHeader.FrameLength),
			Packets:    1,
			Timestamp:  time.Now().Unix(),
			Samplerate: uint64(fs.FlowSampleHeader.SamplingRate),
		}

		if fl.IntIn == "" {
			fl.IntIn += fmt.Sprintf("%d", fs.FlowSampleHeader.InputIf)
		}

		if fl.IntOut == "" {
			fl.IntOut += fmt.Sprintf("%d", fs.FlowSampleHeader.OutputIf)
		}

		if fs.ExtendedRouterData != nil {
			nh, err := bnet.IPFromBytes([]byte(fs.ExtendedRouterData.NextHop))
			if err == nil {
				fl.NextHop = nh
			}
		}

		if fs.ExtendedSwitchData != nil {
			fl.IntIn += fmt.Sprintf(".%d", fs.ExtendedSwitchData.IncomingVLAN)
			fl.IntOut += fmt.Sprintf(".%d", fs.ExtendedSwitchData.OutgoingVLAN)
		}

		sfs.processEthernet(agentStr, ether.EtherType, fs, fl)
		sfs.aggregator.ingress <- fl
	}
}

func (sfs *SflowServer) processEthernet(agentStr string, ethType uint16, fs *sflow.FlowSample, fl *flow.Flow) {
	if ethType == packet.EtherTypeIPv4 {
		sfs.processIPv4Packet(agentStr, fs, fl)
	} else if ethType == packet.EtherTypeIPv6 {
		sfs.processIPv6Packet(agentStr, fs, fl)
	} else if ethType == packet.EtherTypeARP || ethType == packet.EtherTypeLACP {
		return
	} else if ethType == packet.EtherTypeIEEE8021Q {
		sfs.processDot1QPacket(agentStr, fs, fl)
	} else {
		sfs.flowUnknownEtherType.WithLabelValues(agentStr).Inc()
		log.Debugf("Unknown EtherType: 0x%x", ethType)
	}
}

func (sfs *SflowServer) processDot1QPacket(agentStr string, fs *sflow.FlowSample, fl *flow.Flow) {
	dot1q, err := packet.DecodeDot1Q(fs.Data, fs.DataLen)
	if err != nil {
		sfs.flowDot1qDecodeErrors.WithLabelValues(agentStr).Inc()
		log.WithError(err).Debug("Unable to decode dot1q header")
	}
	fs.Data = unsafe.Pointer(uintptr(fs.Data) - packet.SizeOfDot1Q)
	fs.DataLen -= uint32(packet.SizeOfDot1Q)

	sfs.processEthernet(agentStr, dot1q.EtherType, fs, fl)
}

func (sfs *SflowServer) processIPv4Packet(agentStr string, fs *sflow.FlowSample, fl *flow.Flow) {
	fl.Family = 4
	ipv4, err := packet.DecodeIPv4(fs.Data, fs.DataLen)
	if err != nil {
		sfs.flowIPv4DecodeErrors.WithLabelValues(agentStr).Inc()
		log.WithError(err).Debug("Unable to decode IPv4 packet")
	}
	fs.Data = unsafe.Pointer(uintptr(fs.Data) - packet.SizeOfIPv4Header)
	fs.DataLen -= uint32(packet.SizeOfIPv4Header)

	fl.SrcAddr, _ = bnet.IPFromBytes(convert.Reverse(ipv4.SrcAddr[:]))
	fl.DstAddr, _ = bnet.IPFromBytes(convert.Reverse(ipv4.DstAddr[:]))
	fl.Protocol = uint8(ipv4.Protocol)
	switch ipv4.Protocol {
	case packet.TCP:
		if err := getTCP(fs.Data, fs.DataLen, fl); err != nil {
			sfs.flowTCPDecodeErros.WithLabelValues(agentStr).Inc()
			log.WithError(err).Debug("Unable to decode TCP")
		}
	case packet.UDP:
		if err := getUDP(fs.Data, fs.DataLen, fl); err != nil {
			sfs.flowUDPDecodeErros.WithLabelValues(agentStr).Inc()
			log.WithError(err).Debug("Unable to decode UDP")
		}
	}
}

func (sfs *SflowServer) processIPv6Packet(agentStr string, fs *sflow.FlowSample, fl *flow.Flow) {
	fl.Family = 6
	ipv6, err := packet.DecodeIPv6(fs.Data, fs.DataLen)
	if err != nil {
		sfs.flowIPv6DecodeErrors.WithLabelValues(agentStr).Inc()
		log.WithError(err).Debug("Unable to decode IPv6 packet")
	}
	fs.Data = unsafe.Pointer(uintptr(fs.Data) - packet.SizeOfIPv6Header)
	fs.DataLen -= uint32(packet.SizeOfIPv6Header)

	fl.SrcAddr, _ = bnet.IPFromBytes(convert.Reverse(ipv6.SrcAddr[:]))
	fl.DstAddr, _ = bnet.IPFromBytes(convert.Reverse(ipv6.DstAddr[:]))
	fl.Protocol = uint8(ipv6.NextHeader)
	switch ipv6.NextHeader {
	case packet.TCP:
		if err := getTCP(fs.Data, fs.DataLen, fl); err != nil {
			sfs.flowTCPDecodeErros.WithLabelValues(agentStr).Inc()
			log.WithError(err).Debug("Unable to decode TCP")
		}
	case packet.UDP:
		if err := getUDP(fs.Data, fs.DataLen, fl); err != nil {
			sfs.flowUDPDecodeErros.WithLabelValues(agentStr).Inc()
			log.WithError(err).Debug("Unable to decode UDP")
		}
	}
}

func getUDP(udpPtr unsafe.Pointer, length uint32, fl *flow.Flow) error {
	udp, err := packet.DecodeUDP(udpPtr, length)
	if err != nil {
		return errors.Wrap(err, "Unable to decode UDP datagram")
	}

	fl.SrcPort = udp.SrcPort
	fl.DstPort = udp.DstPort

	return nil
}

func getTCP(tcpPtr unsafe.Pointer, length uint32, fl *flow.Flow) error {
	tcp, err := packet.DecodeTCP(tcpPtr, length)
	if err != nil {
		return errors.Wrap(err, "Unable to decode TCP segment")
	}

	fl.SrcPort = tcp.SrcPort
	fl.DstPort = tcp.DstPort

	return nil
}

// Dump dumps a flow on the screen
func Dump(fl *flow.Flow) {
	fmt.Printf("--------------------------------\n")
	fmt.Printf("Flow dump:\n")
	fmt.Printf("Agent: %d\n", fl.Agent)
	fmt.Printf("Family: %d\n", fl.Family)
	fmt.Printf("SrcAddr: %s\n", fl.SrcAddr.String())
	fmt.Printf("DstAddr: %s\n", fl.DstAddr.String())
	fmt.Printf("Protocol: %d\n", fl.Protocol)
	fmt.Printf("NextHop: %s\n", fl.NextHop.String())
	fmt.Printf("IntIn: %d\n", fl.IntIn)
	fmt.Printf("IntOut: %d\n", fl.IntOut)
	fmt.Printf("Packets: %d\n", fl.Packets)
	fmt.Printf("Bytes: %d\n", fl.Size)
	fmt.Printf("--------------------------------\n")
}
