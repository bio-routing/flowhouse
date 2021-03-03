package ipfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEnterprise(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     *TemplateRecord
		expected bool
	}{
		{
			name: "test #1",
			tmpl: &TemplateRecord{
				Type: 0,
			},
			expected: false,
		},
		{
			name: "test #2",
			tmpl: &TemplateRecord{
				Type: 65535,
			},
			expected: true,
		},
		{
			name: "test #3",
			tmpl: &TemplateRecord{
				Type: 32768,
			},
			expected: true,
		},
		{
			name: "test #4",
			tmpl: &TemplateRecord{
				Type: 32767,
			},
			expected: false,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.tmpl.isEnterprise(), test.name)
	}
}
