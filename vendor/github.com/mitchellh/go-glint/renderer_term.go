package glint

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/containerd/console"
	"github.com/gookit/color"
	"github.com/morikuni/aec"
	sshterm "golang.org/x/crypto/ssh/terminal"

	"github.com/mitchellh/go-glint/flex"
)

// TerminalRenderer renders output to a terminal. It expects the Output set
// to be a TTY. This will use ANSI escape codes to redraw.
type TerminalRenderer struct {
	// Output is where to write to. This should be a TTY.
	Output io.Writer

	// Rows, Cols are the dimensions of the terminal. If these are not set
	// (zero), then we will auto-detect the size of the output if it is a TTY.
	// If the values are still zero, nothing will be rendered.
	Rows, Cols uint
}

func (r *TerminalRenderer) LayoutRoot() *flex.Node {
	// If we don't have a writer set, then don't render anything.
	if r.Output == nil {
		return nil
	}

	// Setup our dimensions
	cols := r.Cols
	rows := r.Rows
	if cols == 0 || rows == 0 {
		if f, ok := r.Output.(*os.File); ok && sshterm.IsTerminal(int(f.Fd())) {
			if c, err := console.ConsoleFromFile(f); err == nil {
				if sz, err := c.Size(); err == nil {
					rows = uint(sz.Height)
					cols = uint(sz.Width)
				}
			}
		}
	}

	// Render nothing if we're going to have any zero dimensions
	if cols == 0 || rows == 0 {
		return nil
	}

	// Setup our node
	node := flex.NewNode()
	node.StyleSetWidth(float32(cols))
	node.Context = &termRootContext{
		Rows: rows,
		Cols: cols,
	}

	return node
}

func (r *TerminalRenderer) RenderRoot(root, prev *flex.Node) {
	w := r.Output
	rootCtx := root.Context.(*termRootContext)

	// Draw into a buffer first. This minimizes the time we spend with
	// a blank screen.
	var buf bytes.Buffer
	var sr StringRenderer
	sr.renderTree(&buf, root, -1, color.IsSupportColor())
	rootCtx.Buf = &buf

	if prev != nil {
		// If the previous draw was a terminal and the output was identical,
		// then we do nothing.
		prevCtx, ok := prev.Context.(*termRootContext)
		if ok &&
			prevCtx != nil &&
			prevCtx.Buf != nil &&
			prevCtx.Rows == rootCtx.Rows &&
			prevCtx.Cols == rootCtx.Cols &&
			bytes.Equal(prevCtx.Buf.Bytes(), buf.Bytes()) {
			return
		}

		// Remove what we last drew. If what we last drew is greater than the number
		// of rows then we need to clear the screen.
		height := uint(prev.LayoutGetHeight())
		if height == 0 {
			// If our previous render height is zero that means that everything
			// was finalized and we need to start on a new line.
			fmt.Fprintf(w, "\n")
		} else {
			if height <= rootCtx.Rows {
				// Delete current line
				fmt.Fprint(w, b.Column(0).EraseLine(aec.EraseModes.All).ANSI)

				// Delete n lines above
				for i := uint(0); i < height-1; i++ {
					fmt.Fprint(w, b.Up(1).Column(0).EraseLine(aec.EraseModes.All).ANSI)
				}
			} else {
				fmt.Fprint(w, b.EraseDisplay(aec.EraseModes.All).EraseDisplay(aec.EraseMode(3)).Position(0, 0).ANSI)
			}
		}
	}

	// Draw our current output. We wrap buf in a bytes.Reader so we don't
	// consume the bytes since we'll reuse them in a future call.
	io.Copy(w, bytes.NewReader(buf.Bytes()))
}

func (r *TerminalRenderer) Close() error {
	fmt.Fprintln(r.Output, "")
	return nil
}

type termRootContext struct {
	Rows, Cols uint
	Buf        *bytes.Buffer
}

var b = aec.EmptyBuilder
