package screen

type ScreenCell struct {
	val   rune
	pen   *ScreenPen
	extra []rune
}

func (s *ScreenCell) Value() (rune, []rune) {
	return s.val, s.extra
}

func (s *ScreenCell) Pen() *ScreenPen {
	return s.pen
}

func (s *ScreenCell) reset(r rune, pen *ScreenPen) error {
	s.val = r
	s.pen = pen
	s.extra = nil
	return nil
}

func (s *ScreenCell) resetTo(x *ScreenCell) {
	s.val = x.val
	s.pen = x.pen
	s.extra = nil

	for _, a := range x.extra {
		s.extra = append(s.extra, a)
	}
}

func (s *ScreenCell) addExtra(r rune) error {
	s.extra = append(s.extra, r)
	return nil
}

func NewBuffer(rows, cols int) *Buffer {
	buf := &Buffer{
		rows:  rows,
		cols:  cols,
		lines: make([]*line, rows),
	}

	for i := 0; i < rows; i++ {
		buf.lines[i] = &line{
			cells: make([]ScreenCell, cols),
		}
	}

	return buf
}

type line struct {
	cells []ScreenCell

	used         int
	continuation bool
}

func (l *line) Len() int {
	return l.used
}

func (l *line) resize(sz int) {
	if len(l.cells) >= sz {
		return
	}

	cells := make([]ScreenCell, sz)

	copy(cells, l.cells)

	l.cells = cells
}

func (l *line) Runes() []rune {
	r := make([]rune, 0, l.used)

	for _, c := range l.cells {
		if c.val == 0 {
			r = append(r, ' ')
		} else {
			r = append(r, c.val)
		}
	}

	return r
}

type Buffer struct {
	rows, cols int
	lines      []*line
}

func (b *Buffer) getLine(row int) *line {
	l := b.lines[row]
	if l == nil {
		l = &line{
			cells: make([]ScreenCell, b.cols),
		}

		b.lines[row] = l
	}

	return l
}

func (b *Buffer) injectLine(row int, data []ScreenCell) {
	for i := len(data); i < b.cols; i++ {
		data = append(data, ScreenCell{})
	}

	lines := make([]*line, len(b.lines))

	copy(lines, b.lines[1:row])
	lines[row] = &line{cells: data, used: len(data)}
	copy(lines[row+1:], b.lines[row:])

	b.lines = lines
}

func (b *Buffer) getCell(row, col int) *ScreenCell {
	line := b.getLine(row)
	return &line.cells[col]
}

func (b *Buffer) setCell(row, col int, cell ScreenCell) {
	line := b.getLine(row)

	if col+1 > line.used {
		line.used = col + 1
	}

	line.cells[col] = cell
}

func (b *Buffer) moveInRow(row, start, dest, cols int) {
	line := b.getLine(row)

	if start < dest {
		end := start + cols - 1
		for i := dest + cols - 1; i >= dest; i-- {
			line.cells[i] = line.cells[end]
			end--
		}
	} else {
		for i, cell := range line.cells[start : start+cols] {
			line.cells[dest+i] = cell
		}

		line.used = start + cols
	}
}

func (b *Buffer) moveBetweenRows(row, rowDest, start, cols int) {
	src := b.getLine(row)
	dst := b.getLine(rowDest)

	used := start + cols

	for i := start; i < used; i++ {
		dst.cells[i] = src.cells[i]
		src.cells[i].reset(0, nil)
	}

	if dst.used < used {
		dst.used = used
	}
}

func (b *Buffer) eraseInRow(row, start, cols int) {
	line := b.getLine(row)

	for i := start; i < start+cols; i++ {
		line.cells[i].reset(0, nil)
	}
}
