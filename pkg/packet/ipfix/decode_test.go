package ipfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected *Packet
		wantFail bool
	}{
		{
			name: "Template",
			input: []byte{
				0x00, 0x0a, // Version
				0x00, 0x24, // Length
				0x68, 0x3d, 0x5a, 0xc1, // Timestamp
				0x34, 0x0f, 0xb8, 0x58, // FlowSequence
				0x00, 0x08, 0x00, 0x01, // Observation Domain ID
				0x00, 0x02, // FlowSet ID = 2 = template
				0x00, 0x14, // FlowSet length
				0x01, 0x00, // Template ID
				0x00, 0x03, // Field count
				0x00, 0x08, 0x00, 0x04,
				0x00, 0x0c, 0x00, 0x04,
				0x00, 0x05, 0x00, 0x01,
			},
			expected: &Packet{
				Header: &Header{
					Version:        10,
					Length:         36,
					ExportTime:     1748851393,
					DomainID:       524289,
					SequenceNumber: 873445464,
				},
				Templates: []*TemplateRecords{
					{
						Header: &TemplateRecordHeader{
							TemplateID: 256,
							FieldCount: 3,
						},
						Records: []*TemplateRecord{
							{
								Type:   8,
								Length: 4,
							},
							{
								Type:   12,
								Length: 4,
							},
							{
								Type:   5,
								Length: 1,
							},
						},
					},
				},
				OptionsTemplateRecords: make([]*OptionsTemplateRecords, 0),
			},
		},
		{
			name: "Options Template",
			input: []byte{
				0x00, 0x0a, // Version
				0x00, 0x46, // Length
				0x68, 0x3d, 0x5a, 0xc1, // Timestamp
				0x00, 0x00, 0x46, 0xe9, // Sequence Number
				0x00, 0x08, 0x00, 0x00, // Observation Domain ID
				0x00, 0x03, // FlowSetID
				0x00, 0x36, // FlowSet Length
				0x02, 0x00, // Template ID = 512
				0x00, 0x0b, // Total Field Count
				0x00, 0x01, // Scope Field Count
				0x00, 0x90, 0x00, 0x04,
				0x00, 0x29, 0x00, 0x08,
				0x00, 0x2a, 0x00, 0x08,
				0x00, 0xa0, 0x00, 0x08,
				0x00, 0x82, 0x00, 0x04,
				0x00, 0x83, 0x00, 0x10,
				0x00, 0x22, 0x00, 0x04,
				0x00, 0x24, 0x00, 0x02,
				0x00, 0x25, 0x00, 0x02,
				0x00, 0xd6, 0x00, 0x01,
				0x00, 0xd7, 0x00, 0x01,
			},
			expected: &Packet{
				Header: &Header{
					Version:        10,
					Length:         70,
					ExportTime:     1748851393,
					DomainID:       524288,
					SequenceNumber: 18153,
				},
				Templates: make([]*TemplateRecords, 0),
				OptionsTemplateRecords: []*OptionsTemplateRecords{
					{
						Header: &OptionsTemplateRecordHeader{
							TemplateID:      512,
							TotelFieldCount: 11,
							ScopeFieldCound: 1,
						},
						Records: []*TemplateRecord{
							{
								Type:   144,
								Length: 4,
							},
							{
								Type:   41,
								Length: 8,
							},
							{
								Type:   42,
								Length: 8,
							},
							{
								Type:   160,
								Length: 8,
							},
							{
								Type:   130,
								Length: 4,
							},
							{
								Type:   131,
								Length: 16,
							},
							{
								Type:   34,
								Length: 4,
							},
							{
								Type:   36,
								Length: 2,
							},
							{
								Type:   37,
								Length: 2,
							},
							{
								Type:   214,
								Length: 1,
							},
							{
								Type:   215,
								Length: 1,
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		p, err := Decode(test.input)
		if err == nil && test.wantFail {
			t.Errorf("unexpected success for %q", test.name)
			continue
		}

		if err != nil && !test.wantFail {
			t.Errorf("unexpected failure for %q: %v", test.name, err)
			continue
		}

		p.Buffer = nil
		assert.Equalf(t, test.expected, p, test.name)
	}
}
