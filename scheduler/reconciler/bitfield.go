package reconciler

import (
	"errors"
)

var ErrInvariantViolatedBitfield = errors.New("maximum index for bitfield is 63")

// bitfield aliases uint64 so that it fits into a machine word and is easily
// comparable
type bitfield uint64

const (
	BitJobVersionsMatch  bitfield = 1<<63 | 1<<62
	BitJobNondestructive bitfield = 0<<63 | 1<<62
	BitJobDestructive    bitfield = 0<<63 | 0<<62

	BitAllocComplete bitfield = 1<<61 | 0<<60 | 0<<59 | 0<<58 | 0<<57 | 0<<56
	BitAllocRunning  bitfield = 0<<61 | 1<<60 | 0<<59 | 0<<58 | 0<<57 | 0<<56
	BitAllocPending  bitfield = 0<<61 | 0<<60 | 1<<59 | 0<<58 | 0<<57 | 0<<56
	BitAllocUnknown  bitfield = 0<<61 | 0<<60 | 0<<59 | 1<<58 | 0<<57 | 0<<56
	BitAllocFailed   bitfield = 0<<61 | 0<<60 | 0<<59 | 0<<58 | 1<<57 | 0<<56
	BitAllocLost     bitfield = 0<<61 | 0<<60 | 0<<59 | 0<<58 | 0<<57 | 1<<56

	BitNoReschedule    bitfield = 1<<55 | 0<<54
	BitRescheduleNow   bitfield = 0<<54 | 1<<55
	BitRescheduleLater bitfield = 0<<55 | 0<<54

	BitNodeIsUntainted  bitfield = 1<<53 | 0<<52 | 0<<51
	BitNodeDraining     bitfield = 0<<53 | 1<<52 | 0<<51
	BitNodeDisconnected bitfield = 0<<53 | 0<<52 | 1<<51
	BitNodeDown         bitfield = 0<<53 | 0<<52 | 0<<51

	//	BitAllocAwaitingReconnect bitfield = 1 << 60
)

func (b *bitfield) set(index bitfield) {
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
