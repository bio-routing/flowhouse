package frontend

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatPrefixCondition(t *testing.T) {
	tests := []struct {
		name      string
		pfx       string
		fieldName string
		expected  string
		wantFail  bool
	}{
		{
			name:      "Test #1",
			pfx:       "8.8.8.0/24",
			fieldName: "src_pfx",
			expected:  "src_pfx_addr = IPv4ToIPv6(IPv4StringToNum('8.8.8.0')) AND src_pfx_len = 24",
			wantFail:  false,
		},
		{
			name:      "Test #2",
			pfx:       "2001:db8::/48",
			fieldName: "src_pfx",
			expected:  "src_pfx_addr = IPv6StringToNum('2001:DB8:0:0:0:0:0:0') AND src_pfx_len = 48",
			wantFail:  false,
		},
		{
			name:      "Test #3",
			pfx:       "2001:db8::/48XXX",
			fieldName: "src_pfx",
			wantFail:  true,
		},
	}

	for _, test := range tests {
		res, err := formatPrefixCondition(test.fieldName, test.pfx)
		if test.wantFail && err == nil {
			t.Errorf("Unexpected success for test %s", test.name)
			continue
		}

		if !test.wantFail && err != nil {
			t.Errorf("Unexpected failure for test %s", test.name)
			continue
		}

		assert.Equal(t, test.expected, res, test.name)
	}
}
