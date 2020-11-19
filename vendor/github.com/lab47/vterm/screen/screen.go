package screen

import (
	"bytes"
	"errors"
	"os"
	"sync"

	"github.com/lab47/vterm/state"
)

type CellReader interface {
	GetCell(row, col int) *ScreenCell
}

type Updates interface {
	DamageDone(r state.Rect, cr CellReader) error
	MoveCursor(p state.Pos) error
	SetTermProp(attr state.TermAttr, val interface{}) error
	Output(data []byte) error
	StringEvent(kind string, data []byte) error
}

type ScrollBack interface {
	AddScrollBack(row []rune) error
}

type Screen struct {
	rows, cols int

	pen *ScreenPen

	buffers []*Buffer
	buffer  *Buffer

	mu sync.Mutex

	updates Updates
	scroll  ScrollBack
}

var _ state.Output = &Screen{}

func NewScreen(rows, cols int, updates Updates) (*Screen, error) {
	screen := &Screen{
		rows:    rows,
		cols:    cols,
		updates: updates,

		buffer: NewBuffer(rows, cols),
		pen:    &ScreenPen{},
	}

	if sb, ok := updates.(ScrollBack); ok {
		screen.scroll = sb
	}

	return screen, nil
}

func (s *Screen) WriteToFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close()

	var buf bytes.Buffer

	for _, line := range s.buffer.lines {
		buf.Reset()

		for i, cell := range line.cells {
			if i >= s.cols {
				continue
			}

			if cell.val == 0 || cell.val == ' ' {
				buf.WriteByte('.')
			} else {
				buf.WriteRune(cell.val)
			}
		}

		buf.WriteByte('\n')

		buf.WriteTo(f)
	}

	return nil
}

type cellReader struct {
	s *Screen
}

func (cr cellReader) GetCell(row, col int) *ScreenCell {
	return cr.s.buffer.getCell(row, col)
}

func (s *Screen) getCell(row, col int) *ScreenCell {
	return s.buffer.getCell(row, col)
}

func (s *Screen) setCell(row, col int, cell ScreenCell) {
	s.buffer.setCell(row, col, cell)
}

func (s *Screen) RowString(row int) string {
	var buf bytes.Buffer

	line := s.buffer.lines[row]
	for i, cell := range line.cells {
		if i >= s.cols {
			continue
		}

		if cell.val == 0 || cell.val == ' ' {
			buf.WriteByte('.')
		} else {
			buf.WriteRune(cell.val)
		}
	}

	return buf.String()
}

var ErrOutOfBounds = errors.New("position of out bounds")

func (s *Screen) GetCell(row, col int) *ScreenCell {
	s.mu.Lock()
	defer s.mu.Unlock()

	if row < 0 || row >= s.rows || col < 0 || col >= s.cols {
		return nil
	}

	return s.getCell(row, col)
}

func (s *Screen) damagePos(p state.Pos) error {
	return s.damageRect(state.Rect{Start: p, End: p})
}

func (s *Screen) damageRect(r state.Rect) error {
	return s.updates.DamageDone(r, cellReader{s})
}

func (s *Screen) MoveCursor(pos state.Pos) error {
	return s.updates.MoveCursor(pos)
}

func (s *Screen) SetCell(pos state.Pos, val state.CellRune) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setCell(pos.Row, pos.Col, ScreenCell{val: val.Rune, pen: s.pen})

	return s.damagePos(pos)

	// todo use width
}

func (s *Screen) AppendCell(pos state.Pos, r rune) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cell := s.getCell(pos.Row, pos.Col)
	if cell == nil {
		return nil
	}

	err := cell.addExtra(r)
	if err != nil {
		return err
	}

	return s.damagePos(pos)
}

func (s *Screen) ClearRect(r state.Rect) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for row := r.Start.Row; row < s.rows && row <= r.End.Row; row++ {
		for col := r.Start.Col; col <= r.End.Col; col++ {
			cell := s.getCell(row, col)
			cell.reset(0, s.pen)
		}
	}

	return s.damageRect(r)
}

func (s *Screen) slideRectRight(r state.Rect, dist int) error {
	for row := r.Start.Row; row <= r.End.Row; row++ {
		start := r.Start.Col
		dest := r.Start.Col + dist

		s.buffer.moveInRow(row, start, dest, dist)
		s.buffer.eraseInRow(row, start, dist)
	}

	return nil
}

func (s *Screen) slideRectLeft(r state.Rect, dist int) error {
	for row := r.Start.Row; row <= r.End.Row; row++ {
		start := r.Start.Col
		dest := r.Start.Col - dist

		s.buffer.moveInRow(row, start, dest, dist)
		s.buffer.eraseInRow(row, start, dist)
	}

	return nil
}

func (s *Screen) slideRectDown(r state.Rect, dist int) error {
	cols := r.End.Col - r.Start.Col + 1

	for row := r.End.Row; row >= r.Start.Row; row-- {
		s.buffer.moveBetweenRows(row, row+dist, r.Start.Col, cols)
	}

	for row := r.Start.Row; row < r.Start.Row+dist; row++ {
		s.buffer.eraseInRow(row, r.Start.Col, cols)
	}

	return nil
}

func (s *Screen) slideRectUp(r state.Rect, dist int) error {
	if r.Start.Row == 1 && r.Start.Col == 0 &&
		r.End.Col == s.cols-1 &&
		s.scroll != nil {

		for i := 0; i < dist; i++ {
			line := s.buffer.getLine(i).Runes()
			s.scroll.AddScrollBack(line)
		}
	}

	cols := r.End.Col - r.Start.Col + 1

	for row := r.Start.Row; row <= r.End.Row; row++ {
		s.buffer.moveBetweenRows(row, row-dist, r.Start.Col, cols)
	}

	for row := r.End.Row; row < r.End.Row+dist; row++ {
		s.buffer.eraseInRow(row, r.Start.Col, cols)
	}

	return nil
}

func (s *Screen) ScrollRect(r state.ScrollRect) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch r.Direction {
	case state.ScrollRight:
		sr := r.Rect
		sr.End.Col -= r.Distance

		err := s.slideRectRight(sr, r.Distance)
		if err != nil {
			return err
		}
	case state.ScrollLeft:
		sr := r.Rect
		sr.Start.Col += r.Distance

		err := s.slideRectLeft(sr, r.Distance)
		if err != nil {
			return err
		}
	case state.ScrollDown:
		sr := r.Rect
		sr.End.Row -= r.Distance

		err := s.slideRectDown(sr, r.Distance)
		if err != nil {
			return err
		}
	case state.ScrollUp:
		sr := r.Rect
		sr.Start.Row += r.Distance

		err := s.slideRectUp(sr, r.Distance)
		if err != nil {
			return err
		}
	default:
		return nil
	}

	return s.damageRect(r.Rect)
}

func (s *Screen) Output(data []byte) error {
	return s.updates.Output(data)
}

func (s *Screen) SetTermProp(prop state.TermAttr, val interface{}) error {
	return s.updates.SetTermProp(prop, val)
}

func (s *Screen) SetPenProp(prop state.PenAttr, val interface{}, ps state.PenState) error {
	s.pen = &ScreenPen{PenState: ps}
	return nil
}

func (s *Screen) StringEvent(kind string, data []byte) error {
	return s.updates.StringEvent(kind, data)
}
