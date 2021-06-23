package glint

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/mitchellh/go-glint/flex"
)

// StringRenderer renders output to a string builder. This will clear
// the builder on each frame render. The StringRenderer is primarily meant
// for testing components.
type StringRenderer struct {
	// Builder is the strings builder to write to. If this is nil then
	// it will be created on first render.
	Builder *strings.Builder

	// Width is a fixed width to set for the root node. If this isn't
	// set then a width of 80 is arbitrarily used.
	Width uint
}

func (r *StringRenderer) LayoutRoot() *flex.Node {
	width := r.Width
	if width == 0 {
		width = 80
	}

	node := flex.NewNode()
	node.StyleSetWidth(float32(width))
	return node
}

func (r *StringRenderer) RenderRoot(root, prev *flex.Node) {
	if r.Builder == nil {
		r.Builder = &strings.Builder{}
	}

	// Reset our builder
	r.Builder.Reset()

	// Draw
	r.renderTree(r.Builder, root, -1, false)
}

func (r *StringRenderer) renderTree(final io.Writer, parent *flex.Node, lastRow int, color bool) {
	var buf bytes.Buffer
	for _, child := range parent.Children {
		// Ignore children with a zero height
		if child.LayoutGetHeight() == 0 {
			continue
		}

		// If we're on a different row than last time then we draw a newline.
		thisRow := int(child.LayoutGetTop())
		if lastRow >= 0 && thisRow > lastRow {
			buf.WriteByte('\n')
		}
		lastRow = thisRow

		// Get our node context. If we don't have one then we're a container
		// and we render below.
		ctx, ok := child.Context.(*TextNodeContext)
		if !ok {
			r.renderTree(&buf, child, lastRow, color)
		} else {
			text := ctx.Text
			if color {
				text = styleRender(ctx.Context, text)
			}

			// Draw our text
			fmt.Fprint(&buf, text)
		}
	}

	// We've finished drawing our main content. If we have any paddings/margins
	// we have to draw these now into our buffer.
	leftMargin := int(parent.LayoutGetMargin(flex.EdgeLeft))
	rightMargin := int(parent.LayoutGetMargin(flex.EdgeRight))
	leftPadding := int(parent.LayoutGetPadding(flex.EdgeLeft))
	rightPadding := int(parent.LayoutGetPadding(flex.EdgeRight))

	// NOTE(mitchellh): this is not an optimal way to do this. This was a
	// get-it-done-fast implementation. We should swing back around at some
	// point and rewrite this with less allocations and copying.
	lines := bytes.Split(buf.Bytes(), newline)
	for i, line := range lines {
		final.Write(bytes.Repeat(space, leftMargin+leftPadding))
		final.Write(line)
		final.Write(bytes.Repeat(space, rightMargin+rightPadding))
		if i < len(lines)-1 {
			final.Write(newline)
		}
	}
}

var (
	space   = []byte(" ")
	newline = []byte("\n")
)
