package ipfix

import (
	"unsafe"
)

// OptionsTemplateSetID is the set ID reserved for options template sets
const OptionsTemplateSetID = 3

// OptionsTemplateRecordHeader represents the header of a options template record
type OptionsTemplateRecordHeader struct {
	ScopeFieldCound uint16

	// Totel number of fields in this Otions Template Record. Because a Template FlowSet
	// usually contains multiple Template Records, this field allows the
	// Collector to determine the end of the current Template Record and
	// the start of the next.
	TotelFieldCount uint16

	// Each of the newly generated Template Records is given a unique
	// Template ID. This uniqueness is local to the Observation Domain that
	// generated the Template ID. Template IDs of Data FlowSets are numbered
	// from 256 to 65535.
	TemplateID uint16
}

var sizeOfOptionsTemplateRecordHeader = unsafe.Sizeof(OptionsTemplateRecordHeader{})

// OptionsTemplateRecords is a single template that describes structure of an options Flow Record
// (actual Netflow data).
type OptionsTemplateRecords struct {
	Header *OptionsTemplateRecordHeader

	// List of fields in this Template Record.
	Records []*TemplateRecord
}
