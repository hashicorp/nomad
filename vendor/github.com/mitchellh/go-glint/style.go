package glint

import (
	"context"

	"github.com/gookit/color"
)

// Style applies visual styles to this component and any children. This
// can be used to set a foreground color, for example, to a set of components.
func Style(inner Component, opts ...StyleOption) Component {
	c := &styleComponent{inner: inner}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// styleRender is used internally to apply styles to the given string. The
// ctx should be the same context given when Body was called on this
// component.
func styleRender(ctx context.Context, v string) string {
	value, _ := ctx.Value(styleCtxKey).([]*styleComponent)
	for _, s := range value {
		v = s.render(v)
	}

	return v
}

type styleComponent struct {
	inner            Component
	fgColor, bgColor colorizer
	style            []color.Color
}

func (c *styleComponent) Body(ctx context.Context) Component {
	// Add our style to the list of styles. We have to use copy here
	// so we don't append to a parent.
	old, _ := ctx.Value(styleCtxKey).([]*styleComponent)
	value := make([]*styleComponent, len(old), len(old)+1)
	copy(value, old)
	value = append(value, c)
	return Context(c.inner, styleCtxKey, value)
}

func (c *styleComponent) render(v string) string {
	if c.bgColor != nil {
		v = c.bgColor.Sprint(v)
	}
	if c.fgColor != nil {
		v = c.fgColor.Sprint(v)
	}
	v = color.Style(c.style).Sprint(v)

	return v
}

type styleCtxKeyType struct{}

var styleCtxKey = styleCtxKeyType{}

// StyleOption is an option that can be set when creating Text components.
type StyleOption func(t *styleComponent)

// Color sets the color by name. The supported colors are listed below.
//
// black, red, green, yellow, blue, magenta, cyan, white, darkGray,
// lightRed, lightGreen, lightYellow, lightBlue, lightMagenta, lightCyan,
// lightWhite.
func Color(name string) StyleOption {
	return func(t *styleComponent) {
		if c, ok := color.FgColors[name]; ok {
			t.fgColor = c
		}
		if c, ok := color.ExFgColors[name]; ok {
			t.fgColor = c
		}
	}
}

// ColorHex sets the foreground color by hex code. The value can be
// in formats AABBCC, #AABBCC, 0xAABBCC.
func ColorHex(v string) StyleOption {
	return func(t *styleComponent) {
		t.fgColor = color.HEX(v)
	}
}

// ColorRGB sets the foreground color by RGB values.
func ColorRGB(r, g, b uint8) StyleOption {
	return func(t *styleComponent) {
		t.fgColor = color.RGB(r, g, b)
	}
}

// BGColor sets the color by name. The supported colors are listed below.
//
// black, red, green, yellow, blue, magenta, cyan, white, darkGray,
// lightRed, lightGreen, lightYellow, lightBlue, lightMagenta, lightCyan,
// lightWhite.
func BGColor(name string) StyleOption {
	return func(t *styleComponent) {
		if c, ok := color.BgColors[name]; ok {
			t.bgColor = c
		}
		if c, ok := color.ExBgColors[name]; ok {
			t.bgColor = c
		}
	}
}

// BGColorHex sets the foreground color by hex code. The value can be
// in formats AABBCC, #AABBCC, 0xAABBCC.
func BGColorHex(v string) StyleOption {
	return func(t *styleComponent) {
		t.bgColor = color.HEX(v, true)
	}
}

// BGColorRGB sets the foreground color by RGB values.
func BGColorRGB(r, g, b uint8) StyleOption {
	return func(t *styleComponent) {
		t.bgColor = color.RGB(r, g, b, true)
	}
}

// Bold sets the text to bold.
func Bold() StyleOption {
	return func(t *styleComponent) {
		t.style = append(t.style, color.OpBold)
	}
}

// Italic sets the text to italic.
func Italic() StyleOption {
	return func(t *styleComponent) {
		t.style = append(t.style, color.OpItalic)
	}
}

// Underline sets the text to be underlined.
func Underline() StyleOption {
	return func(t *styleComponent) {
		t.style = append(t.style, color.OpUnderscore)
	}
}

type colorizer interface {
	Sprint(...interface{}) string
}
