package screen

import "github.com/lab47/vterm/state"

type Tx struct {
	s      *Screen
	damage *state.Rect
}

func (tx *Tx) SetCell(pos state.Pos, val state.CellRune) error {
	tx.s.setCell(pos.Row, pos.Col, ScreenCell{val: val.Rune, pen: tx.s.pen})
	if tx.damage == nil {
		tx.damage = &state.Rect{Start: pos, End: pos}
	} else {
		if pos.Col > tx.damage.End.Col {
			tx.damage.End.Col = pos.Col
		}

		if pos.Row > tx.damage.End.Row {
			tx.damage.End.Row = pos.Row
		}
	}

	return nil
}

func (tx *Tx) AppendCell(pos state.Pos, r rune) error {
	cell := tx.s.getCell(pos.Row, pos.Col)
	if cell == nil {
		return nil
	}

	err := cell.addExtra(r)
	if err != nil {
		return err
	}

	if tx.damage == nil {
		tx.damage = &state.Rect{Start: pos, End: pos}
	} else {
		if pos.Col > tx.damage.End.Col {
			tx.damage.End.Col = pos.Col
		}

		if pos.Row > tx.damage.End.Row {
			tx.damage.End.Row = pos.Row
		}
	}

	return nil
}

func (tx *Tx) Close() error {
	tx.s.mu.Unlock()

	if tx.damage == nil {
		return nil
	}

	return tx.s.damageRect(*tx.damage)
}

func (s *Screen) BeginTx() state.ModifyTx {
	s.mu.Lock()

	var tx Tx
	tx.s = s

	return &tx
}
