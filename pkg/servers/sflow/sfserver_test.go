package sflow

import (
	"testing"

	"github.com/bio-routing/tflow2/convert"
)

func TestExtractTrafficClass(t *testing.T) {
	tests := []struct {
		name     string
		input    uint32
		expected uint8
	}{
		{
			name:     "Test case 1",
			input:    convert.Uint32b([]byte{0, 0, 0xff, 0xff}),
			expected: 0,
		},
		{
			name:     "Test case 2",
			input:    convert.Uint32b([]byte{0xcf, 0xf0, 0xac, 0xab}),
			expected: 255,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := extractTrafficClass(test.input)
			if result != test.expected {
				t.Errorf("Expected %d, got %d", test.expected, result)
			}
		})
	}
}
