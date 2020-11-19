package screen

import (
	"github.com/lab47/vterm/state"
)

func (s *Screen) Resize(rows, cols int, lines []state.LineInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// q.Q(rows, cols)

	// buf := NewBuffer(rows, cols)

	if cols > s.cols {
		diff := cols - s.cols

		for row := 0; row < s.rows; row++ {
			if !lines[row].Continuation {
				continue
			}

			src := s.buffer.getLine(row)
			tgt := s.buffer.getLine(row - 1)

			tgt.resize(cols)

			for i, j := 0, s.cols; i < diff; i, j = i+1, j+1 {
				tgt.cells[j] = src.cells[i]
			}

			src.cells = src.cells[diff:]
		}
	} else if s.cols > cols {
		// diff := cols - s.cols

		var prepend []ScreenCell

		for row := 0; row < s.rows; row++ {
			src := s.buffer.getLine(row)

			if len(prepend) > 0 {
				if lines[row].Continuation {
					cells := make([]ScreenCell, len(prepend)+len(src.cells))
					copy(cells, prepend)
					copy(cells[len(prepend):], src.cells)
					src.cells = cells
				} else {
					s.buffer.injectLine(row, prepend)
				}

				prepend = nil
			}

			l := src.Len()

			if l <= cols {
				continue
			}

			prepend = src.cells[cols:l]
			src.cells = src.cells[:cols]
		}
	}

	/*
		if s.cols < cols {

			var outIdx int

			for row := 0; row < s.rows; row++ {
				if !lines[row].Continuation {
					outIdx = row * cols
				}

				for col := 0; col < s.cols; col++ {
					cell := s.getCell(row, col)
					// fmt.Printf("%d.%d => %d.%d (%d) => %d\n", row, col, outIdx/cols, outIdx%cols, outIdx, cell.val)
					buf.cells[outIdx] = *cell
					outIdx++
				}
			}
		} else if s.cols > cols {
			var outIdx int

			for row := 0; row < s.rows; row++ {
				if !lines[row].Continuation {
					outIdx = row * cols
				}

				for col := 0; col < s.cols; col++ {
					cell := s.getCell(row, col)
					// fmt.Printf("%d.%d => %d.%d (%d) => %d\n", row, col, outIdx/cols, outIdx%cols, outIdx, cell.val)
					buf.cells[outIdx] = *cell
					outIdx++
				}
			}
		}
	*/

	s.rows = rows
	s.cols = cols
	// s.buffer = buf

	return nil
}
