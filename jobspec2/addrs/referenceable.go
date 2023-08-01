package addrs

// Referenceable is an interface implemented by all address types that can
// appear as references in configuration language expressions.
type Referenceable interface {
	// referenceableSigil is private to ensure that all Referenceables are
	// implentented in this current package. For now this does nothing.
	referenceableSigil()

	// String produces a string representation of the address that could be
	// parsed as a HCL traversal and passed to ParseRef to produce an identical
	// result.
	String() string
}

// referenceable is an empty struct that implements Referenceable, add it to
// your Referenceable struct so that it can be recognized as such.
type referenceable struct {
}

func (r referenceable) referenceableSigil() {
}
