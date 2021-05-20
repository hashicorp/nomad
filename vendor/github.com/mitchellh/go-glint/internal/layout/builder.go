package layout

import (
	"github.com/mitchellh/go-glint/flex"
)

type SetFunc func(n *flex.Node)

// Builder builds a set of styles to apply to a flex node.
type Builder struct {
	f SetFunc
}

// Raw composes a SetFunc on the builder. This will call the previous
// styles setters first and then call this function.
func (l *Builder) Raw(f SetFunc) *Builder {
	return l.add(f)
}

// Apply sets the styles on the flex node.
func (l *Builder) Apply(node *flex.Node) {
	if l == nil || l.f == nil {
		return
	}

	l.f(node)
}

// add is a helper to add the function to the call chain for this builder.
// This will return a new builder.
func (l *Builder) add(f func(*flex.Node)) *Builder {
	old := l.f
	new := func(n *flex.Node) {
		if old != nil {
			old(n)
		}

		f(n)
	}

	return &Builder{f: new}
}
