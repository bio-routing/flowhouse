// Copyright 2017 EXARING AG. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sflow

import (
	"net"
	"unsafe"
)

// Packet is a decoded representation of a single sflow UDP packet.
type Packet struct {
	// A pointer to the packets headers
	Header       *Header
	headerTop    *headerTop
	headerBottom *headerBottom

	// A slice of pointers to FlowSet. Each element is instance of (Data)FlowSet
	FlowSamples []*FlowSample

	// Buffer is a slice pointing to the original byte array that this packet was decoded from.
	// This field is only populated if debug level is at least 2
	Buffer []byte
}

var (
	sizeOfHeaderTop                = unsafe.Sizeof(headerTop{})
	sizeOfHeaderBottom             = unsafe.Sizeof(headerBottom{})
	sizeOfFlowSampleHeader         = unsafe.Sizeof(FlowSampleHeader{})
	sizeOfExpandedFlowSampleHeader = unsafe.Sizeof(ExpandedFlowSampleHeader{})
	sizeOfRawPacketHeader          = unsafe.Sizeof(RawPacketHeader{})
	sizeofExtendedRouterData       = unsafe.Sizeof(ExtendedRouterData{})
	sizeOfextendedRouterDataTop    = unsafe.Sizeof(extendedRouterDataTop{})
	sizeOfextendedRouterDataBottom = unsafe.Sizeof(extendedRouterDataBottom{})
)

// Header is an sflow version 5 header
type Header struct {
	Version          uint32
	AgentAddressType uint32
	AgentAddress     net.IP
	SubAgentID       uint32
	SequenceNumber   uint32
	SysUpTime        uint32
	NumSamples       uint32
}

type headerTop struct {
	AgentAddressType uint32
	Version          uint32
}

type headerBottom struct {
	NumSamples     uint32
	SysUpTime      uint32
	SequenceNumber uint32
	SubAgentID     uint32
}

// FlowSample is an sflow version 5 flow sample
type FlowSample struct {
	FlowSampleHeader         *FlowSampleHeader
	ExpandedFlowSampleHeader *ExpandedFlowSampleHeader
	RawPacketHeader          *RawPacketHeader
	Data                     unsafe.Pointer
	DataLen                  uint32
	ExtendedRouterData       *ExtendedRouterData
}

// FlowSampleHeader is an sflow version 5 flow sample header
type FlowSampleHeader struct {
	FlowRecord         uint32
	OutputIf           uint32
	InputIf            uint32
	DroppedPackets     uint32
	SamplePool         uint32
	SamplingRate       uint32
	SourceIDClassIndex uint32
	SequenceNumber     uint32
	SampleLength       uint32
	EnterpriseType     uint32
}

// ExpandedFlowSampleHeader is an sflow version 5 flow expanded sample header
type ExpandedFlowSampleHeader struct {
	FlowRecord         uint32
	OutputIf           uint32
	_                  uint32
	InputIf            uint32
	_                  uint32
	DroppedPackets     uint32
	SamplePool         uint32
	SamplingRate       uint32
	SourceIDClassIndex uint32
	_                  uint32
	SequenceNumber     uint32
	SampleLength       uint32
	EnterpriseType     uint32
}

func (e *ExpandedFlowSampleHeader) toFlowSampleHeader() *FlowSampleHeader {
	return &FlowSampleHeader{
		FlowRecord:         e.FlowRecord,
		OutputIf:           e.OutputIf,
		InputIf:            e.InputIf,
		DroppedPackets:     e.DroppedPackets,
		SamplePool:         e.SamplePool,
		SamplingRate:       e.SamplingRate,
		SourceIDClassIndex: e.SourceIDClassIndex,
		SequenceNumber:     e.SequenceNumber,
		SampleLength:       e.SampleLength,
		EnterpriseType:     e.EnterpriseType,
	}
}

// RawPacketHeader is a raw packet header
type RawPacketHeader struct {
	OriginalPacketLength uint32
	PayloadRemoved       uint32
	FrameLength          uint32
	HeaderProtocol       uint32
	FlowDataLength       uint32
	EnterpriseType       uint32
}

type extendedRouterDataTop struct {
	AddressType    uint32
	FlowDataLength uint32
	EnterpriseType uint32
}

type extendedRouterDataBottom struct {
	NextHopDestinationMask uint32
	NextHopSourceMask      uint32
}

// ExtendedRouterData represents sflow version 5 extended router data
type ExtendedRouterData struct {
	NextHopDestinationMask uint32
	NextHopSourceMask      uint32
	NextHop                net.IP
	AddressType            uint32
	FlowDataLength         uint32
	EnterpriseType         uint32
}
