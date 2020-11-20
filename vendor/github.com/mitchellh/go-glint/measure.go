package glint

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/mitchellh/go-glint/flex"
	"github.com/mitchellh/go-wordwrap"
)

// TextNodeContext is the *flex.Node.Context set for all *TextComponent flex nodes.
type TextNodeContext struct {
	// C is the TextComponent represented.
	C *TextComponent

	// The context at the time layout was done
	Context context.Context

	// Text is the rendered text. This is populated after MeasureTextNode
	// is called. Note that this may not fit in the final layout calculations
	// since it is populated on measurement.
	Text string

	// Size is the measurement size returned. This can be used to determine
	// if the text above fits in the final size. Text is guaranteed to fit
	// in this size.
	Size flex.Size
}

func (c *TextNodeContext) Component() Component { return c.C }

// MeasureTextNode implements flex.MeasureFunc and returns the measurements
// for the given node only if the node represents a TextComponent. This is
// the MeasureFunc that is typically used for renderers since all component
// trees terminate in a text node.
//
// The flex.Node must have Context set to TextNodeContext. After calling this,
// fields such as Text and Size will be populated on the node.
func MeasureTextNode(
	node *flex.Node,
	width float32,
	widthMode flex.MeasureMode,
	height float32,
	heightMode flex.MeasureMode,
) flex.Size {
	// If we have no context set then we use the full spacing.
	ctx, ok := node.Context.(*TextNodeContext)
	if !ok || ctx == nil {
		return flex.Size{Width: width, Height: height}
	}

	// Otherwise, we have to render this.
	ctx.Text = ctx.C.Render(uint(height), uint(width))

	// Word wrap and truncate if we're beyond the width limit.
	if !math.IsNaN(float64(width)) && width > 0 {
		ctx.Text = clampTextWidth(
			wordwrap.WrapString(ctx.Text, uint(width)),
			int(width))
	}

	// Truncate height if we have a limit. This is a no-op if it fits.
	if !math.IsNaN(float64(height)) && height > 0 {
		ctx.Text = truncateTextHeight(ctx.Text, int(height))
	}

	// Calculate the size
	ctx.Size = flex.Size{
		Width:  float32(longestLine(ctx.Text)),
		Height: float32(countLines(ctx.Text)),
	}

	// We special case the empty-text case, since this is a height of
	// one and width of zero. If the user wanted no rendering at all they
	// should render nil.
	if ctx.Text == "" {
		ctx.Size.Height = 1
	}

	return ctx.Size
}

func countLines(s string) int {
	count := strings.Count(s, "\n")

	// If the last character isn't a newline, we have to add one since we'll
	// always have one more line than newline characters.
	if len(s) > 0 && s[len(s)-1] != '\n' {
		count++
	}
	return count
}

func longestLine(s string) int {
	longest := 0
	for {
		idx := strings.IndexByte(s, '\n')
		if idx == -1 {
			break
		}

		current := utf8.RuneCountInString(s[:idx])
		if current > longest {
			longest = current
		}

		s = s[idx+1:]
	}

	if longest == 0 {
		return utf8.RuneCountInString(s)
	}

	return longest
}

func truncateTextHeight(s string, height int) string {
	// The way this works is that we iterate through HEIGHT newlines
	// and return up to that point. If we either don't find a newline
	// or we've reached the end of the string, then the string is shorter
	// than the height limit and we return the whole thing.
	idx := 0
	for i := 0; i < height; i++ {
		next := strings.IndexByte(s[idx:], '\n')
		if next == -1 || idx >= len(s) {
			return s
		}

		idx += next + 1
	}

	// This can happen if height == 0
	if idx == 0 {
		return ""
	}

	// Subtract one here because the idx is the last "\n" char
	return s[:idx-1]
}

// clampTextWidth cuts off any lines in s that are longer than width
// characters (not including the newline).
func clampTextWidth(s string, width int) string {
	// If our width is zero just return empty
	if width == 0 {
		return ""
	}

	// NOTE(mitchellh): This loop is really horrible. It is unclear, weirdly
	// repetitive, and just aesthetically gross. But the tests pass and we have
	// good test cases on this. Let's fix this later.
	var b *strings.Builder
	total := 0
	original := s
	for {
		end := false
		idx := strings.IndexByte(s, '\n')
		if idx == -1 {
			idx = len(s)
			end = true
		}

		runeCount := utf8.RuneCountInString(s[:idx])
		if runeCount > width {
			if b == nil {
				b = &strings.Builder{}
				if total > 0 {
					b.WriteString(original[:total])
					b.WriteByte('\n')
				}
			}

			runes := []rune(s)
			b.WriteString(string(runes[:width]))
			if !end {
				b.WriteByte('\n')
			}
		} else if idx > 0 {
			if b != nil {
				b.WriteString(s[:idx])
			}
		}

		if end {
			break
		}

		total += idx
		s = s[idx+1:]
	}

	if b == nil {
		return original
	}

	return b.String()
}
