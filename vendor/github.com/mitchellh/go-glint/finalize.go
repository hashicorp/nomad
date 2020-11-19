package glint

import "context"

// Finalize reutrns a component that will finalize the input component.
// See ComponentFinalizer for documentation on what finalization means.
func Finalize(c Component) Component {
	return &finalizedComponent{
		Component: c,
	}
}

type finalizedComponent struct {
	Component
}

func (c *finalizedComponent) Body(context.Context) Component {
	return c.Component
}
