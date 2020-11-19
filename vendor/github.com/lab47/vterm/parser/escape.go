package parser

type EscapeCommand int

const (
	S7C1T EscapeCommand = iota
	S8C1T
	DECDHL_TOP
	DECDHL_BOTTOM
	DECSWL
	DECDWL
	DECALN
	SCS1
	SCS2
	SCS3
	SCS4
	DECSC
	DECRC
	VT100UP
	DECKPAM
	DECKPNM
	RIS
	LS2
	LS3
	LS1R
	LS2R
	LS3R
)

var EscapeMatcher = map[EscapeCommand][]byte{
	S7C1T:         []byte{' ', 'F'},
	S8C1T:         []byte{' ', 'G'},
	DECDHL_TOP:    []byte{'#', '3'},
	DECDHL_BOTTOM: []byte{'#', '4'},
	DECSWL:        []byte{'#', '5'},
	DECDWL:        []byte{'#', '6'},
	DECALN:        []byte{'#', '8'},
	SCS1:          []byte{'('},
	SCS2:          []byte{')'},
	SCS3:          []byte{'*'},
	SCS4:          []byte{'+'},
	DECSC:         []byte{'7'},
	DECRC:         []byte{'8'},
	VT100UP:       []byte{'<'},
	DECKPAM:       []byte{'='},
	DECKPNM:       []byte{'>'},
	RIS:           []byte{'c'},
	LS2:           []byte{'n'},
	LS3:           []byte{'o'},
	LS1R:          []byte{'~'},
	LS2R:          []byte{'}'},
	LS3R:          []byte{'|'},
}
