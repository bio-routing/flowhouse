package packet

import (
	"fmt"
	"unsafe"
)

var (
	// SizeOfDot1Q is the size of an Dot1Q header in bytes
	SizeOfDot1Q = unsafe.Sizeof(Dot1Q{})
)

// Dot1Q represents an 802.1q header
type Dot1Q struct {
	EtherType uint16
	TCI       uint16
}

// DecodeDot1Q decodes an 802.1q header
func DecodeDot1Q(raw unsafe.Pointer, length uint32) (*Dot1Q, error) {
	if SizeOfEthernetII > uintptr(length) {
		return nil, fmt.Errorf("Frame is too short: %d", length)
	}

	ptr := unsafe.Pointer(uintptr(raw) - SizeOfDot1Q)
	dot1qHeader := (*Dot1Q)(ptr)

	return dot1qHeader, nil
}
