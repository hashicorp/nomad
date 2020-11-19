package state

type TermAttr int

const (
	TermAttrTitle TermAttr = iota
	TermAttrIconName
	TermAttrReverse
	TermAttrBlink
	TermAttrVisible
	TermAttrMouse
	TermAttrAltScreen
	TermAttrOSC
)

//go:generate stringer -type=TermAttr
