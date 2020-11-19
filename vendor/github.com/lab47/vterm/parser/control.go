package parser

type ControlCommand int

const (
	_    ControlCommand = iota
	BELX ControlCommand = 0x7
	BS   ControlCommand = 0x8
	HT   ControlCommand = 0x9
	LF   ControlCommand = 0xa
	VT   ControlCommand = 0xb
	FF   ControlCommand = 0xc
	CR   ControlCommand = 0xd
	LSI  ControlCommand = 0xe
	LS0  ControlCommand = 0xf
)
