package agent

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/lab47/vterm/parser"
	"github.com/lab47/vterm/screen"
	"github.com/lab47/vterm/state"
	"github.com/mitchellh/go-glint"
)

type glintTerm struct {
	mu sync.Mutex

	w      io.Writer
	scr    *screen.Screen
	ctx    context.Context
	cancel func()

	output        [][]rune
	height, width int

	wg       sync.WaitGroup
	parseErr error

	scrollback [][]rune

	full bool
}

func (t *glintTerm) Body(ctx context.Context) glint.Component {
	t.mu.Lock()
	defer t.mu.Unlock()

	var cs []glint.Component

	if t.full {
		for _, row := range t.scrollback {
			s := strings.TrimRightFunc(string(row), unicode.IsSpace)
			cs = append(cs, glint.Layout(glint.Text(" │ "), glint.Text(s)).Row())
		}
	}

	for _, row := range t.output {
		cs = append(cs, glint.Layout(
			glint.Text(" │ "),
			glint.Style(
				glint.Text(strings.TrimRightFunc(string(row), unicode.IsSpace)),
				glint.Color("lightBlue"),
			),
		).Row())
	}

	return glint.Fragment(cs...)
}

func (t *glintTerm) DamageDone(r state.Rect, cr screen.CellReader) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for row := r.Start.Row; row <= r.End.Row; row++ {
		for col := r.Start.Col; col <= r.End.Col; col++ {
			cell := cr.GetCell(row, col)

			for len(t.output) <= row {
				t.output = append(t.output, make([]rune, t.width))
			}

			if cell == nil {
				t.output[row][col] = ' '
			} else {
				val, _ := cell.Value()

				if val == 0 {
					t.output[row][col] = ' '
				} else {
					t.output[row][col] = val
				}
			}
		}
	}

	return nil
}

func (t *glintTerm) MoveCursor(p state.Pos) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) SetTermProp(attr state.TermAttr, val interface{}) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) Output(data []byte) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) StringEvent(kind string, data []byte) error {
	// Ignore them.
	return nil
}

func newGlintTerm(ctx context.Context, height, width int) (*glintTerm, error) {
	term := &glintTerm{
		height: height,
		width:  width,
	}

	scr, err := screen.NewScreen(height, width, term)
	if err != nil {
		return nil, err
	}

	term.scr = scr

	st, err := state.NewState(height, width, scr)
	if err != nil {
		return nil, err
	}

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	term.w = w

	prs, err := parser.NewParser(r, st)
	if err != nil {
		return nil, err
	}

	term.ctx, term.cancel = context.WithCancel(ctx)

	term.wg.Add(1)
	go func() {
		defer term.wg.Done()

		err := prs.Drive(term.ctx)
		if err != nil && err != context.Canceled {
			term.parseErr = err
		}
	}()

	return term, nil
}

func (t *glintTerm) Write(b []byte) (int, error) {
	return t.w.Write(b)
}

func (t *glintTerm) AddScrollBack(line []rune) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.scrollback = append(t.scrollback, line)
	return nil
}

func (t *glintTerm) showFull() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.full = true
}

var _ screen.ScrollBack = (*glintTerm)(nil)

func (t *glintTerm) Close() error {
	t.cancel()
	t.wg.Wait()
	return t.parseErr
}
