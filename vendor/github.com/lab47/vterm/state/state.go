package state

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/lab47/vterm/parser"
)

type Pos struct {
	Row, Col int
}

type Rect struct {
	Start, End Pos
}

func (r Rect) Height() int {
	return (r.End.Row - r.Start.Row) + 1
}

func (r Rect) Width() int {
	return (r.End.Col - r.Start.Col) + 1
}

// Split the rectangle into two rectangles as columns within the
// original rectangle. The size of the right rectangle is a +perc+ percentage
// of the original
func (s Rect) SplitColumns(perc float32) (Rect, Rect) {
	width := float32(s.End.Col - s.Start.Col)

	left := s
	right := s

	right.Start.Col = right.End.Col - int(width*(perc/100.0))

	left.End.Col = right.Start.Col - 1

	return left, right
}

// Split the rectangle into 2 parts as though we were
// to run a line down the middle vertically in the original.
func (s Rect) SplitEvenColumns() (Rect, Rect) {
	width := (s.End.Col - s.Start.Col) + 1

	left := s
	right := s

	left.End.Col = left.Start.Col + ((width / 2) - 1)

	right.Start.Col = left.End.Col + 1

	return left, right
}

// Split the rectangle into two rectangles as rows within the
// original rectangle. The size of the bottom rectangle is a +perc+ percentage
// of the original
func (s Rect) SplitRows(perc float32) (Rect, Rect) {
	height := float32(s.End.Row - s.Start.Row)

	top := s
	bottom := s

	bottom.Start.Row = bottom.End.Row - int(height*(perc/100.0))

	top.End.Row = bottom.Start.Row - 1

	return top, bottom
}

// Split the rectangle into 2 parts as though we were
// to run a line down the middle horizontally in the original.
func (s Rect) SplitEvenRows() (Rect, Rect) {
	height := (s.End.Row - s.Start.Row) + 1

	top := s
	bottom := s

	top.End.Row = top.Start.Row + ((height / 2) - 1)

	bottom.Start.Row = top.End.Row + 1

	return top, bottom
}

func (s Rect) ScrollUp(dist int) ScrollRect {
	x := ScrollRect{Rect: s}
	x.Direction = ScrollUp
	x.Distance = dist
	return x
}

func (s Rect) ScrollDown(dist int) ScrollRect {
	x := ScrollRect{Rect: s}
	x.Direction = ScrollDown
	x.Distance = dist
	return x
}

func (s Rect) ScrollLeft(dist int) ScrollRect {
	x := ScrollRect{Rect: s}
	x.Direction = ScrollLeft
	x.Distance = dist
	return x
}

func (s Rect) ScrollRight(dist int) ScrollRect {
	x := ScrollRect{Rect: s}
	x.Direction = ScrollRight
	x.Distance = dist
	return x
}

type ScrollDirection int

//go:generate stringer -type=ScrollDirection

const (
	ScrollNone  ScrollDirection = iota // don't scroll
	ScrollUp                           // move the content at the top of the rect to the bottom
	ScrollDown                         // move the content at the bottom of the rect to the top
	ScrollRight                        // move the content on the right side of the rect to the left side
	ScrollLeft                         // move the content on the left side of the rect to the right side
)

type ScrollRect struct {
	Rect
	Direction ScrollDirection
	Distance  int
}

type CellRune struct {
	Rune  rune
	Width int
}

type ModifyTx interface {
	SetCell(pos Pos, val CellRune) error
	AppendCell(pos Pos, val rune) error
	Close() error
}

type Output interface {
	MoveCursor(pos Pos) error
	SetCell(pos Pos, val CellRune) error
	BeginTx() ModifyTx
	AppendCell(pos Pos, r rune) error
	ClearRect(r Rect) error
	ScrollRect(s ScrollRect) error
	Output(data []byte) error
	SetTermProp(prop TermAttr, val interface{}) error
	SetPenProp(prop PenAttr, val interface{}, ps PenState) error
	StringEvent(kind string, data []byte) error
	Resize(rows, cols int, lines []LineInfo) error
}

type modes struct {
	insert          bool
	newline         bool
	cursor          bool
	origin          bool
	autowrap        bool
	leftrightmargin bool
	report_focus    bool
	bracketpaste    bool
	altscreen       bool
}

const (
	MouseNone int = iota
	MouseClick
	MouseDrag
	MouseMove
)

const (
	MouseX10 int = iota
	MouseUTF8
	MouseSGR
	MouseRXVT
)

type LineInfo struct {
	Continuation bool
}

type State struct {
	Debug bool
	Id    string

	rows, cols int
	cursor     Pos
	atPhantom  bool
	pen        PenState
	output     Output

	lastPos  Pos
	tabStops []bool

	modes         modes
	mouseProtocol int
	savedCursor   Pos

	scrollregion struct {
		top, bottom int
	}

	lineInfo     []LineInfo
	deferNewline bool
}

var _ parser.EventHandler = &State{}

func NewState(rows, cols int, output Output) (*State, error) {
	screen := &State{
		rows:     rows,
		cols:     cols,
		output:   output,
		tabStops: make([]bool, cols),
		lineInfo: make([]LineInfo, rows),
	}

	err := screen.Reset()
	if err != nil {
		return nil, err
	}

	return screen, nil
}

func (s *State) Reset() error {
	s.modes = modes{autowrap: true}
	for col := 0; col < s.cols; col++ {
		if col%8 == 0 {
			s.tabStops[col] = true
		} else {
			s.tabStops[col] = false
		}
	}
	s.modes.newline = true

	s.scrollregion.top = 0
	s.scrollregion.bottom = -1

	s.pen.fgColor = DefaultColor{}
	s.pen.bgColor = DefaultColor{}

	return nil
}

func (s *State) Resize(rows, cols int) error {
	s.rows = rows
	s.cols = cols
	return s.output.Resize(rows, cols, s.lineInfo)
}

func (s *State) HandleEvent(gev parser.Event) error {
	if s.deferNewline {
		s.cursor = s.lineFeed(s.cursor, false)
	}

	switch ev := gev.(type) {
	case *parser.TextEvent:
		return s.writeData(ev)
	case parser.ControlEvent:
		return s.handleControl(byte(ev))
	case *parser.CSIEvent:
		return s.handleCSI(ev)
	case *parser.StringEvent:
		return s.handleString(ev)
	case *parser.OSCEvent:
		return s.handleOSC(ev)
	case parser.ResizeEvent:
		err := s.Resize(ev.Rows, ev.Cols)
		if ev.Confirm != nil {
			ev.Confirm <- err
		}

		return err
	case *parser.EscapeEvent:
		return s.handleEsc(ev)
	default:
		return fmt.Errorf("unhandled event type: %T", ev)
	}
}

func (s *State) scrollBounds() (int, int) {
	bottom := s.scrollregion.bottom
	if bottom <= -1 {
		bottom = s.rows - 1
	}

	return s.scrollregion.top, bottom
}

func (s *State) lineFeed(p Pos, canDefer bool) Pos {
	switch {
	case p.Row < 0:
		p.Row = 0
	case p.Row >= s.rows-1:
		if canDefer && !s.deferNewline {
			s.deferNewline = true
			return p
		}

		s.deferNewline = false

		s.output.ScrollRect(ScrollRect{
			Rect: Rect{
				Start: Pos{0, 0},
				End:   Pos{s.rows - 1, s.cols - 1},
			},
			Direction: ScrollUp,
			Distance:  1,
		})
		p.Row = s.rows - 1
	case p.Row < s.rows-1:
		p.Row++
	}

	return p
}

func (s *State) setCursor(p Pos) {
	if s.modes.origin {
		switch {
		case p.Row < s.scrollregion.top:
			p.Row = s.scrollregion.top
		case p.Row >= s.scrollregion.bottom:
			p.Row = s.scrollregion.bottom
		}
	} else {
		switch {
		case p.Row < 0:
			p.Row = 0
		case p.Row >= s.rows:
			p.Row = s.rows - 1
		}
	}

	switch {
	case p.Col < 0:
		p.Col = 0
	case p.Col >= s.cols:
		p.Col = s.cols - 1
	}

	s.updateCursor(p, true)
}

func (s *State) updateCursor(p Pos, cancelPhantom bool) {
	if s.cursor == p {
		return
	}

	s.cursor = p

	if cancelPhantom {
		s.atPhantom = false
	}

	s.output.MoveCursor(p)
}

func (s *State) writeData(ev *parser.TextEvent) error {
	defer ev.Recycle()
	data := ev.Text

	tx := s.output.BeginTx()

	for len(data) > 0 {
		r, sz := utf8.DecodeRune(data)

		data = data[sz:]

		if unicode.In(r, unicode.Diacritic) {
			err := tx.AppendCell(s.lastPos, r)
			if err != nil {
				return err
			}

			continue
		}

		pos := s.cursor
		width := 1 // TODO find the real width and use it.

		if s.atPhantom || pos.Col+width > s.cols {
			tx.Close()

			pos = s.lineFeed(pos, false)
			pos.Col = 0
			s.atPhantom = false
			if pos.Row < len(s.lineInfo) {
				s.lineInfo[pos.Row].Continuation = true
			}

			tx = s.output.BeginTx()
		}

		s.lastPos = pos

		err := tx.SetCell(pos, CellRune{r, 1})
		if err != nil {
			return err
		}

		if pos.Col+width >= s.cols {
			if s.modes.autowrap {
				s.atPhantom = true
			}
		} else {
			pos.Col += width
		}

		s.cursor = pos
	}

	tx.Close()

	return s.output.MoveCursor(s.cursor)
}

func (s *State) emitBell() error {
	return nil
}

func (s *State) handleControl(control byte) error {
	pos := s.cursor

	switch control {
	case 0x7: // BEL
		return s.emitBell()
	case 0x8: // BS
		if pos.Col > 0 {
			pos.Col--
		}

	case 0x9: // HT
		pos.Col++
		for pos.Col < s.cols {
			if s.tabStops[pos.Col] {
				break
			}

			pos.Col++
		}
	case 0xa, 0xb, 0xc:
		pos = s.lineFeed(pos, true)

		if s.modes.newline {
			pos.Col = 0
		}
	case 0xd:
		pos.Col = 0
	case 0x84: // IND
		pos = s.lineFeed(pos, true)
	case 0x85: // NEL
		pos = s.lineFeed(pos, true)
		pos.Col = 0
	case 0x88: // HTS
		s.tabStops[pos.Col] = true
		return nil

	case 0x8d: // synthesized by the parser when handling esc'd 7-bit sequences
		if pos.Row > s.scrollregion.top {
			pos.Row--
		} else {
			start := Pos{s.scrollregion.top, 0}
			end := Pos{s.scrollregion.bottom, s.cols - 1}

			return s.output.ScrollRect(Rect{start, end}.ScrollDown(1))
		}
	}

	s.updateCursor(pos, true)
	return nil
}

var csiHandlers = map[parser.CSICommand]func(*State, *parser.CSIEvent) error{
	parser.CUU: (*State).cursorUp,
	parser.VPB: (*State).cursorUp,
	parser.CUD: (*State).cursorDown,
	parser.VPR: (*State).cursorDown,
	parser.CUF: (*State).cursorForward,
	parser.HPR: (*State).cursorForward,
	parser.CUB: (*State).cursorBackward,
	parser.HPB: (*State).cursorBackward,
	parser.CNL: (*State).cursorNextLine,
	parser.CPL: (*State).cursorPrevLine,
	parser.CHA: (*State).cursorMoveCol,
	parser.HPA: (*State).cursorMoveCol,
	parser.CUP: (*State).cursorMove,
	parser.HVP: (*State).cursorMove,
	parser.VPA: (*State).cursorMoveRow,
	parser.CHT: (*State).cursorTabForward,
	parser.CBT: (*State).cursorTabBackward,

	parser.ICH: (*State).insertBlankChars,
	parser.ED:  (*State).eraseDisplay,
	parser.EL:  (*State).eraseLine,
	parser.IL:  (*State).insertLines,
	parser.DL:  (*State).deleteLines,
	parser.DCH: (*State).deleteChars,
	parser.SU:  (*State).scrollUp,
	parser.SD:  (*State).scrollDown,
	parser.ECH: (*State).eraseChars,

	parser.DA:    (*State).emitDeviceAttributes,
	parser.DA_LT: (*State).emitDeviceAttributes2,

	parser.TBC: (*State).clearTabStop,

	parser.SM:   (*State).setMode,
	parser.SM_Q: (*State).setDecMode,

	parser.RM:   (*State).removeMode,
	parser.RM_Q: (*State).removeDecMode,

	parser.SGR: (*State).selectGraphics,

	parser.DSR:   (*State).statusReport,
	parser.DSR_Q: (*State).statusReportDec,

	parser.DECSTR: (*State).softReset,

	parser.DECSTBM: (*State).setTopBottomMargin,

	parser.MOUSE: (*State).mouseEvent,
}

func (s *State) handleCSI(ev *parser.CSIEvent) error {
	cmd := ev.CSICommand()
	f, ok := csiHandlers[cmd]
	if !ok {
		return fmt.Errorf("unhandled CSI command: (%s) %x", cmd, ev.Command)
	}

	defer ev.Recycle()

	return f(s, ev)
}

func (s *State) cursorMove(ev *parser.CSIEvent) error {
	var pos Pos

	if len(ev.Args) > 0 && ev.Args[0] > 0 {
		pos.Row = ev.Args[0] - 1
	}

	if len(ev.Args) > 1 && ev.Args[1] > 0 {
		pos.Col = ev.Args[1] - 1
	}

	if pos.Row < 0 {
		pos.Row = 0
	} else if pos.Row >= s.rows {
		pos.Row = s.rows - 1
	}

	if pos.Col < 0 {
		pos.Col = 0
	} else if pos.Col >= s.cols {
		pos.Col = s.cols - 1
	}

	if s.modes.origin {
		pos.Row += s.scrollregion.top
	}

	s.setCursor(pos)

	return nil
}

func (s *State) cursorMoveCol(ev *parser.CSIEvent) error {
	pos := s.cursor

	if len(ev.Args) > 0 && ev.Args[0] > 0 {
		pos.Col = ev.Args[0] - 1
	} else {
		pos.Col = 0
	}

	if pos.Col < 0 {
		pos.Col = 0
	} else if pos.Col >= s.cols {
		pos.Col = s.cols - 1
	}

	s.setCursor(pos)

	return nil
}

func (s *State) cursorMoveRow(ev *parser.CSIEvent) error {
	pos := s.cursor

	row := 1

	if len(ev.Args) > 0 {
		row = ev.Args[0]
	}

	if row > 0 {
		pos.Row = row - 1
	}

	if pos.Row < 0 {
		pos.Row = 0
	} else if pos.Row >= s.rows {
		pos.Row = s.rows - 1
	}

	s.setCursor(pos)

	return nil
}

func (s *State) cursorForward(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Col += inc

	if pos.Col >= s.cols {
		pos.Col = s.cols - 1
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorBackward(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Col -= inc

	if pos.Col < 0 {
		pos.Col = 0
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorTabForward(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	for i := 0; i < inc; i++ {
		for pos.Col < s.cols {
			pos.Col++

			if s.tabStops[pos.Col] {
				break
			}
		}
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorTabBackward(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	for i := 0; i < inc; i++ {
		for pos.Col > 0 {
			pos.Col--

			if s.tabStops[pos.Col] {
				break
			}
		}
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorUp(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Row -= inc

	if pos.Row < 0 {
		pos.Row = 0
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorDown(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Row += inc

	if pos.Row >= s.rows {
		pos.Row = s.rows - 1
	}

	s.setCursor(pos)
	return nil
}

func (s *State) cursorNextLine(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Row += inc

	if pos.Row >= s.rows {
		pos.Row = s.rows - 1
	}

	pos.Col = 0

	s.setCursor(pos)
	return nil
}

func (s *State) cursorPrevLine(ev *parser.CSIEvent) error {
	pos := s.cursor

	inc := 1

	if len(ev.Args) > 0 && ev.Args[0] != 0 {
		inc = ev.Args[0]
	}

	pos.Row -= inc

	if pos.Row < 0 {
		pos.Row = 0
	}

	pos.Col = 0

	s.setCursor(pos)
	return nil
}

func (s *State) insertBlankChars(ev *parser.CSIEvent) error {
	start := s.cursor

	end := start

	end.Col = s.cols - 1

	dist := 1
	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	return s.output.ScrollRect(Rect{start, end}.ScrollRight(dist))
}

func (s *State) eraseDisplay(ev *parser.CSIEvent) error {
	mode := 0

	if len(ev.Args) > 0 {
		mode = ev.Args[0]
	}

	// TODO support the ? leader to indicate the DEC selective erase, which
	// only erases characters that were previously defined by DECSCA.

	switch mode {
	case 0: // from cursor to end of display
		start := s.cursor
		end := start

		end.Col = s.cols - 1

		if start.Col > 0 {
			err := s.output.ClearRect(Rect{start, end})
			if err != nil {
				return err
			}
			start.Row++
		}

		start.Col = 0

		end.Row = s.rows - 1

		return s.output.ClearRect(Rect{start, end})
	case 1: // from start to cursor
		start := Pos{0, 0}

		end := s.cursor

		end.Row--
		end.Col = s.cols - 1

		err := s.output.ClearRect(Rect{start, end})
		if err != nil {
			return err
		}

		start = s.cursor
		end = start

		start.Col = 0

		return s.output.ClearRect(Rect{start, end})
	case 2: // the whole display
		start := Pos{0, 0}
		end := Pos{s.rows - 1, s.cols - 1}
		return s.output.ClearRect(Rect{start, end})
	}

	return nil
}

func (s *State) eraseLine(ev *parser.CSIEvent) error {
	mode := 0

	if len(ev.Args) > 0 {
		mode = ev.Args[0]
	}

	// TODO support the ? leader to indicate the DEC selective erase, which
	// only erases characters that were previously defined by DECSCA.

	start := s.cursor
	end := start

	switch mode {
	case 0: // from cursor to end of line
		end.Col = s.cols - 1
	case 1: // from start to cursor
		start.Col = 0
	case 2: // the whole display
		start.Col = 0
		end.Col = s.cols - 1
	default:
		return nil
	}

	return s.output.ClearRect(Rect{start, end})
}

func (s *State) insertLines(ev *parser.CSIEvent) error {
	var dist int

	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	if dist == 0 {
		dist = 1
	}

	start := s.cursor
	start.Col = 0

	_, bottom := s.scrollBounds()

	end := Pos{bottom, s.cols - 1}

	return s.output.ScrollRect(Rect{start, end}.ScrollDown(dist))
}

func (s *State) deleteLines(ev *parser.CSIEvent) error {
	var dist int

	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	if dist == 0 {
		dist = 1
	}

	start := s.cursor
	start.Col = 0

	_, bottom := s.scrollBounds()

	end := Pos{bottom, s.cols - 1}

	return s.output.ScrollRect(Rect{start, end}.ScrollUp(dist))
}

func (s *State) deleteChars(ev *parser.CSIEvent) error {
	start := s.cursor

	end := start

	end.Col = s.cols - 1

	dist := 1
	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	return s.output.ScrollRect(Rect{start, end}.ScrollLeft(dist))
}

func (s *State) scrollUp(ev *parser.CSIEvent) error {
	top, bottom := s.scrollBounds()

	start := Pos{top, 0}
	end := Pos{bottom, s.cols - 1}

	var dist int
	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	if dist == 0 {
		dist = 1
	}

	return s.output.ScrollRect(Rect{start, end}.ScrollUp(dist))
}

func (s *State) scrollDown(ev *parser.CSIEvent) error {
	top, bottom := s.scrollBounds()

	start := Pos{top, 0}
	end := Pos{bottom, s.cols - 1}

	var dist int
	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	if dist == 0 {
		dist = 1
	}

	return s.output.ScrollRect(Rect{start, end}.ScrollDown(dist))
}

func (s *State) eraseChars(ev *parser.CSIEvent) error {
	start := s.cursor

	var dist int

	if len(ev.Args) > 0 {
		dist = ev.Args[0]
	}

	if dist == 0 {
		dist = 1
	}

	end := start
	end.Col += (dist - 1)

	return s.output.ClearRect(Rect{start, end})
}

func (s *State) emitDeviceAttributes(ev *parser.CSIEvent) error {
	return s.output.Output([]byte("\x9b?1;2c"))
}

func (s *State) emitDeviceAttributes2(ev *parser.CSIEvent) error {
	return s.output.Output([]byte("\x9b>0;100;0c"))
}

func (s *State) clearTabStop(ev *parser.CSIEvent) error {
	var mode int

	if len(ev.Args) > 0 {
		mode = ev.Args[0]
	}

	switch mode {
	case 0:
		s.tabStops[s.cursor.Col] = false
	case 3:
		s.tabStops = make([]bool, s.cols)
	}

	return nil
}

func (s *State) setMode(ev *parser.CSIEvent) error {
	if len(ev.Args) == 0 {
		return nil
	}

	mode := ev.Args[0]

	switch mode {
	case 4:
		s.modes.insert = true
	case 20:
		s.modes.newline = true
	}

	return nil
}

func (s *State) setDecMode(ev *parser.CSIEvent) error {
	if len(ev.Args) == 0 {
		return nil
	}

	mode := ev.Args[0]

	switch mode {
	case 1:
		s.modes.cursor = true
	case 5:
		return s.output.SetTermProp(TermAttrReverse, true)
	case 6:
		s.modes.origin = true
		s.updateCursor(Pos{0, 0}, true)
	case 7:
		s.modes.autowrap = true
	case 12:
		return s.output.SetTermProp(TermAttrBlink, true)
	case 25:
		return s.output.SetTermProp(TermAttrVisible, true)
	case 69:
		s.modes.leftrightmargin = true
	case 1000:
		return s.output.SetTermProp(TermAttrMouse, MouseClick)
	case 1002:
		return s.output.SetTermProp(TermAttrMouse, MouseDrag)
	case 1003:
		return s.output.SetTermProp(TermAttrMouse, MouseMove)
	case 1004:
		s.modes.report_focus = true
	case 1005:
		s.mouseProtocol = MouseUTF8
	case 1006:
		s.mouseProtocol = MouseSGR
	case 1015:
		s.mouseProtocol = MouseRXVT
	case 1047:
		return s.output.SetTermProp(TermAttrAltScreen, true)
	case 1048:
		s.savedCursor = s.cursor
	case 1049:
		s.savedCursor = s.cursor
		return s.output.SetTermProp(TermAttrAltScreen, true)
	case 2004:
		s.modes.bracketpaste = true
	}

	return nil
}

func (s *State) removeMode(ev *parser.CSIEvent) error {
	if len(ev.Args) == 0 {
		return nil
	}

	mode := ev.Args[0]

	switch mode {
	case 4:
		s.modes.insert = false
	case 20:
		s.modes.newline = false
	}

	return nil
}

func (s *State) removeDecMode(ev *parser.CSIEvent) error {
	if len(ev.Args) == 0 {
		return nil
	}

	mode := ev.Args[0]

	switch mode {
	case 1:
		s.modes.cursor = false
	case 5:
		return s.output.SetTermProp(TermAttrReverse, false)
	case 6:
		s.modes.origin = false
	case 7:
		s.modes.autowrap = false
	case 12:
		return s.output.SetTermProp(TermAttrBlink, false)
	case 25:
		return s.output.SetTermProp(TermAttrVisible, false)
	case 69:
		s.modes.leftrightmargin = false
	case 1000:
		return s.output.SetTermProp(TermAttrMouse, MouseNone)
	case 1002:
		return s.output.SetTermProp(TermAttrMouse, MouseNone)
	case 1003:
		return s.output.SetTermProp(TermAttrMouse, MouseNone)
	case 1004:
		s.modes.report_focus = false
	case 1005:
		s.mouseProtocol = MouseX10
	case 1006:
		s.mouseProtocol = MouseX10
	case 1015:
		s.mouseProtocol = MouseX10
	case 1047:
		return s.output.SetTermProp(TermAttrAltScreen, false)
	case 1048:
		s.updateCursor(s.savedCursor, true)
	case 1049:
		s.updateCursor(s.savedCursor, true)
		return s.output.SetTermProp(TermAttrAltScreen, false)
	case 2004:
		s.modes.bracketpaste = false
	}

	return nil
}

func (s *State) statusReport(ev *parser.CSIEvent) error {
	var which int

	if len(ev.Args) > 0 {
		which = ev.Args[0]
	}

	switch which {
	case 5:
		return s.output.Output([]byte("\x9b0n"))
	case 6:
		return s.output.Output([]byte(fmt.Sprintf("\x9b%d;%dR", s.cursor.Row+1, s.cursor.Col+1)))
	}

	return nil
}

func (s *State) statusReportDec(ev *parser.CSIEvent) error {
	var which int

	if len(ev.Args) > 0 {
		which = ev.Args[0]
	}

	switch which {
	case 5:
		return s.output.Output([]byte("\x9b?0n"))
	case 6:
		return s.output.Output([]byte(fmt.Sprintf("\x9b?%d;%dR", s.cursor.Row+1, s.cursor.Col+1)))
	}

	return nil
}

func (s *State) softReset(ev *parser.CSIEvent) error {
	return s.Reset()
}

func (s *State) setTopBottomMargin(ev *parser.CSIEvent) error {
	var (
		top    = 1
		bottom = -1
	)

	switch len(ev.Args) {
	case 2:
		bottom = ev.Args[1] - 1
		fallthrough
	case 1:
		top = ev.Args[0]
	}

	if top < 1 {
		top = 1
	}

	if top > s.rows {
		top = s.rows
	}

	if bottom > s.rows {
		bottom = s.rows
	}

	s.scrollregion.top = top - 1
	s.scrollregion.bottom = bottom

	return nil
}

func (s *State) handleEsc(ev *parser.EscapeEvent) error {
	if len(ev.Data) == 1 {
		switch ev.Data[0] {
		case 'M':
			pos := s.cursor
			if pos.Row > s.scrollregion.top {
				pos.Row--
			}

			s.updateCursor(pos, true)
		}
	}

	return nil
}
