package glint

import (
	"context"
)

// TextComponent is a Component that renders text.
type TextComponent struct {
	terminalComponent
	f func(rows, cols uint) string
}

// Text creates a TextComponent for static text. The text here will be word
// wrapped automatically based on the width of the terminal.
func Text(v string) *TextComponent {
	return TextFunc(func(rows, cols uint) string { return v })
}

// TextFunc creates a TextComponent for text that is dependent on the
// size of the draw area.
func TextFunc(f func(rows, cols uint) string) *TextComponent {
	return &TextComponent{
		f: f,
	}
}

func (el *TextComponent) Body(context.Context) Component {
	return nil
}

func (el *TextComponent) Render(rows, cols uint) string {
	if el.f == nil {
		return ""
	}

	return el.f(rows, cols)
}
