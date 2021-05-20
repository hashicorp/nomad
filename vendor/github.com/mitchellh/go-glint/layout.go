package glint

import (
	"context"

	"github.com/mitchellh/go-glint/flex"
	"github.com/mitchellh/go-glint/internal/layout"
)

// Layout is used to set layout properties for the child components.
// This can be used similarly to a "div" in HTML with "display: flex" set.
// This component follows the builder pattern for setting layout properties
// such as margins, paddings, etc.
func Layout(inner ...Component) *LayoutComponent {
	return &LayoutComponent{inner: inner, builder: &layout.Builder{}}
}

// LayoutComponent is a component used for layout settings. See Layout.
type LayoutComponent struct {
	inner   []Component
	builder *layout.Builder
}

// Row sets the `flex-direction: row` property.
func (c *LayoutComponent) Row() *LayoutComponent {
	c.builder = c.builder.Raw(func(n *flex.Node) {
		n.StyleSetFlexDirection(flex.FlexDirectionRow)
	})
	return c
}

// MarginLeft sets the `margin-left` property.
func (c *LayoutComponent) MarginLeft(x int) *LayoutComponent {
	c.builder = c.builder.Raw(func(n *flex.Node) {
		n.StyleSetMargin(flex.EdgeLeft, float32(x))
	})
	return c
}

// MarginRight sets the `margin-left` property.
func (c *LayoutComponent) MarginRight(x int) *LayoutComponent {
	c.builder = c.builder.Raw(func(n *flex.Node) {
		n.StyleSetMargin(flex.EdgeRight, float32(x))
	})
	return c
}

// PaddingLeft sets the `margin-left` property.
func (c *LayoutComponent) PaddingLeft(x int) *LayoutComponent {
	c.builder = c.builder.Raw(func(n *flex.Node) {
		n.StyleSetPadding(flex.EdgeLeft, float32(x))
	})
	return c
}

// PaddingRight sets the `margin-left` property.
func (c *LayoutComponent) PaddingRight(x int) *LayoutComponent {
	c.builder = c.builder.Raw(func(n *flex.Node) {
		n.StyleSetPadding(flex.EdgeRight, float32(x))
	})
	return c
}

// Component implementation
func (c *LayoutComponent) Body(context.Context) Component {
	return Fragment(c.inner...)
}

// componentLayout internal implementation.
func (c *LayoutComponent) Layout() *layout.Builder {
	return c.builder
}
