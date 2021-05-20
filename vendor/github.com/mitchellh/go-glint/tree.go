package glint

import (
	"context"

	"github.com/mitchellh/go-glint/flex"
)

func tree(
	ctx context.Context,
	parent *flex.Node,
	c Component,
	finalize bool,
) {
	// Don't do anything with no component
	if c == nil {
		return
	}

	// Fragments don't create a node
	switch c := c.(type) {
	case *contextComponent:
		for i := 0; i < len(c.pairs); i += 2 {
			ctx = context.WithValue(ctx, c.pairs[i], c.pairs[i+1])
		}

		tree(ctx, parent, c.inner, finalize)
		return

	case *fragmentComponent:
		for _, c := range c.List {
			tree(ctx, parent, c, finalize)
		}

		return
	}

	// Setup our node
	node := flex.NewNodeWithConfig(parent.Config)
	parent.InsertChild(node, len(parent.Children))

	// Setup our default context
	parentCtx := &parentContext{C: c}
	node.Context = parentCtx

	// Check if we're finalized and note it
	if _, ok := c.(*finalizedComponent); ok {
		parentCtx.Finalized = true
	}

	// Finalize
	if finalize {
		if c, ok := c.(ComponentFinalizer); ok {
			c.Finalize()
		}
	}

	// Setup a custom layout
	if c, ok := c.(componentLayout); ok {
		c.Layout().Apply(node)
	}

	switch c := c.(type) {
	case *TextComponent:
		node.Context = &TextNodeContext{C: c, Context: ctx}
		node.StyleSetFlexShrink(1)
		node.StyleSetFlexGrow(0)
		node.StyleSetFlexDirection(flex.FlexDirectionRow)
		node.SetMeasureFunc(MeasureTextNode)

	default:
		// If this is not terminal then we nest.
		tree(ctx, node, c.Body(ctx), finalize)
	}

}

type parentContext struct {
	C         Component
	Finalized bool
}

func (c *parentContext) Component() Component { return c.C }

type treeContext interface {
	Component() Component
}
