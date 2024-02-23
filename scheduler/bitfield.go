package scheduler

import (
	"errors"
)

var ErrInvariantViolatedBitfield = errors.New("maximum index for bitfield is 63")

// bitfield aliases uint64 so that it fits into a machine word and is easily
// comparable. The lowest two bytes are reserved for the Alloc Name Index, so
// this leaves 48 bits for the candidate heap index. If we need more than this,
// we'll need to bump up to a bitarray which will be more complex and probably
// slower
type bitfield uint64

func (b *bitfield) set(index int) {
	if index > 63 {
		panic(ErrInvariantViolatedBitfield)
	}
	n := bitfield(uint64(*b) | 1<<index)
	*b = n
}

func (b *bitfield) isSet(index int) bool {
	if index > 63 {
		panic(ErrInvariantViolatedBitfield)
	}
	return (uint64(*b) & (1 << index)) != 0
}

func (b *bitfield) mergeUint16(o uint16) {
	n := bitfield(uint64(*b) | uint64(o))
	*b = n
}
