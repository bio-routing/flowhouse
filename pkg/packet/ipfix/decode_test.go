package ipfix

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTemplate(t *testing.T) {
	tests := []struct {
		name     string
		pkt      *Packet
		end      unsafe.Pointer
		size     uintptr
		expected *Packet
	}{
		{
			name: "Test #1",
			pkt: &Packet{
				Templates: make([]*TemplateRecords, 0),
			},
		},
	}

	for _, test := range tests {
		decodeTemplate(test.pkt, test.end, test.size)
		assert.Equal(t, test.expected, test.pkt, test.name)
	}
}
