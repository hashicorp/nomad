package glint

// Fragment appends multiple components together. A fragment has no layout
// implications, it is as if the set of components were appended directly to
// the parent.
func Fragment(c ...Component) Component {
	return &fragmentComponent{List: c}
}

type fragmentComponent struct {
	terminalComponent

	List []Component
}
