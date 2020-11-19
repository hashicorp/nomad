package parser

type CSICode struct {
	Name        string
	Description string
}

type CSICommand int

func LEADER(l, b byte) CSICommand {
	return (CSICommand(l) << 8) | CSICommand(b)
}

func INTERMED(i, b byte) CSICommand {
	return (CSICommand(i) << 16) | CSICommand(b)
}

var (
	MOUSE    CSICommand = 0x3c
	ICH      CSICommand = 0x40
	CUU      CSICommand = 0x41
	CUD      CSICommand = 0x42
	CUF      CSICommand = 0x43
	CUB      CSICommand = 0x44
	CNL      CSICommand = 0x45
	CPL      CSICommand = 0x46
	CHA      CSICommand = 0x47
	CUP      CSICommand = 0x48
	CHT      CSICommand = 0x49
	ED       CSICommand = 0x4a
	DECSED   CSICommand = LEADER('?', 0x4a)
	EL       CSICommand = 0x4b
	DECSEL   CSICommand = LEADER('?', 0x4b)
	IL       CSICommand = 0x4c
	DL       CSICommand = 0x4d
	DCH      CSICommand = 0x50
	SU       CSICommand = 0x53
	SD       CSICommand = 0x54
	ECH      CSICommand = 0x58
	CBT      CSICommand = 0x5a
	HPA      CSICommand = 0x60
	HPR      CSICommand = 0x61
	REP      CSICommand = 0x62
	DA       CSICommand = 0x63
	DA_LT    CSICommand = LEADER('>', 0x63)
	VPA      CSICommand = 0x64
	VPR      CSICommand = 0x65
	HVP      CSICommand = 0x66
	TBC      CSICommand = 0x67
	SM       CSICommand = 0x68
	SM_Q     CSICommand = LEADER('?', 0x68)
	HPB      CSICommand = 0x6a
	VPB      CSICommand = 0x6b
	RM       CSICommand = 0x6c
	RM_Q     CSICommand = LEADER('?', 0x6c)
	SGR      CSICommand = 0x6d
	DSR      CSICommand = 0x6e
	DSR_Q    CSICommand = LEADER('?', 0x6e)
	DECSTR   CSICommand = LEADER('!', 0x70)
	DECSCUSR CSICommand = INTERMED(' ', 0x71)
	DECSCA   CSICommand = INTERMED('"', 0x71)
	DECSTBM  CSICommand = 0x72
	DECSLRM  CSICommand = 0x73
	DECIC    CSICommand = INTERMED('\'', 0x7D)
	DECDC    CSICommand = INTERMED('\'', 0x7E)
)

func (c CSICommand) String() string {
	if code, ok := CSICodes[c]; ok {
		return code.Name
	}

	return "UNKNOWN"
}

var CSICodes = map[CSICommand]CSICode{
	0x3c:                 {"MOUSE", "XTerm Mouse Event"},
	0x40:                 {"ICH", "ECMA-48 8.3.64"},
	0x41:                 {"CUU", "ECMA-48 8.3.22"},
	0x42:                 {"CUD", "ECMA-48 8.3.19"},
	0x43:                 {"CUF", "ECMA-48 8.3.20"},
	0x44:                 {"CUB", "ECMA-48 8.3.18"},
	0x45:                 {"CNL", "ECMA-48 8.3.12"},
	0x46:                 {"CPL", "ECMA-48 8.3.13"},
	0x47:                 {"CHA", "ECMA-48 8.3.9"},
	0x48:                 {"CUP", "ECMA-48 8.3.21"},
	0x49:                 {"CHT", "ECMA-48 8.3.10"},
	0x4a:                 {"ED", "ECMA-48 8.3.39"},
	LEADER('?', 0x4a):    {"DECSED", "Selective Erase in Display"},
	0x4b:                 {"EL", "ECMA-48 8.3.41"},
	LEADER('?', 0x4b):    {"DECSEL", "Selective Erase in Line"},
	0x4c:                 {"IL", "ECMA-48 8.3.67"},
	0x4d:                 {"DL", "ECMA-48 8.3.32"},
	0x50:                 {"DCH", "ECMA-48 8.3.26"},
	0x53:                 {"SU", "ECMA-48 8.3.147"},
	0x54:                 {"SD", "ECMA-48 8.3.113"},
	0x58:                 {"ECH", "ECMA-48 8.3.38"},
	0x5a:                 {"CBT", "ECMA-48 8.3.7"},
	0x60:                 {"HPA", "ECMA-48 8.3.57"},
	0x61:                 {"HPR", "ECMA-48 8.3.59"},
	0x62:                 {"REP", "ECMA-48 8.3.103"},
	0x63:                 {"DA", "ECMA-48 8.3.24"},
	LEADER('>', 0x63):    {"DA-LT", "DEC secondary Device Attributes"},
	0x64:                 {"VPA", "ECMA-48 8.3.158"},
	0x65:                 {"VPR", "ECMA-48 8.3.160"},
	0x66:                 {"HVP", "ECMA-48 8.3.63"},
	0x67:                 {"TBC", "ECMA-48 8.3.154"},
	0x68:                 {"SM", "ECMA-48 8.3.125"},
	LEADER('?', 0x68):    {"SM-Q", "DEC private mode set"},
	0x6a:                 {"HPB", "ECMA-48 8.3.58"},
	0x6b:                 {"VPB", "ECMA-48 8.3.159"},
	0x6c:                 {"RM", "ECMA-48 8.3.106"},
	LEADER('?', 0x6c):    {"RM-Q", "DEC private mode reset"},
	0x6d:                 {"SGR", "ECMA-48 8.3.117"},
	0x6e:                 {"DSR", "ECMA-48 8.3.35"},
	LEADER('?', 0x6e):    {"DSR-Q", "DECDSR"},
	LEADER('!', 0x70):    {"DECSTR", "DEC soft terminal reset"},
	INTERMED(' ', 0x71):  {"DECSCUSR", "DEC set cursor shape"},
	INTERMED('"', 0x71):  {"DECSCA", "DEC select character protection attribute"},
	0x72:                 {"DECSTBM", "DEC custom"},
	0x73:                 {"DECSLRM", "DEC custom"},
	INTERMED('\'', 0x7D): {"DECIC", "DEC Scroll Screen Up"},
	INTERMED('\'', 0x7E): {"DECDC", "DEC Scroll Screen Down"},
}
