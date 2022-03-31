package encoding

import (
	"encoding/binary"
)

// FromBytes16 turns []byte into uint16
func FromBytes16(data []byte) uint16 {
	return binary.BigEndian.Uint16(data)
}

// ToBytes16 turns a uint16 into []byte len 2
func ToBytes16(in uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, in)
	return buf
}

// FromBytes8 turns a []byte into a uint8.
func FromBytes8(data []byte) uint8 {
	if len(data) == 1 {
		data = []byte{0x00, data[0]}
	}
	// there isn't a Uint8 function
	i16 := binary.BigEndian.Uint16(data)
	return uint8(i16)
}

// ToBytes8 turns uint8 into []byte of len 1 (eg. 8 bits)
func ToBytes8(in uint8) []byte {
	// uint8 is only one byte anyways right
	return []byte{in}
}

// Split32 uint32 to two uint16
func Split32(in uint32) (uint16, uint16) {
	return uint16(in >> 16), uint16(in)
}

// Merge16 two uint16 to uint32
func Merge16(a, b uint16) uint32 {
	return (uint32(a) << 16) + uint32(b)
}

// Split16 uint16 to two uint8
func Split16(in uint16) (uint8, uint8) {
	return uint8(in >> 8), uint8(in)
}

// Merge8 two uint8 to uint16
func Merge8(a, b uint8) uint16 {
	return (uint16(a) << 8) + uint16(b)
}
