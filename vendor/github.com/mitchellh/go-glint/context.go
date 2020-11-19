package glint

// Context is a component type that can be used to set data on the context
// given to Body calls for components that are children of this component.
func Context(inner Component, kv ...interface{}) Component {
	if len(kv)%2 != 0 {
		panic("kv must be set in pairs")
	}

	return &contextComponent{inner: inner, pairs: kv}
}

type contextComponent struct {
	terminalComponent

	inner Component
	pairs []interface{}
}
