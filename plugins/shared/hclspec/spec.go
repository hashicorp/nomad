package hclspec

// ObjectSpec wraps the object and returns a spec.
func ObjectSpec(obj *Object) *Spec {
	return &Spec{
		Block: &Spec_Object{
			Object: obj,
		},
	}
}

// ArraySpec wraps the array and returns a spec.
func ArraySpec(array *Array) *Spec {
	return &Spec{
		Block: &Spec_Array{
			Array: array,
		},
	}
}

// AttrSpec wraps the attr and returns a spec.
func AttrSpec(attr *Attr) *Spec {
	return &Spec{
		Block: &Spec_Attr{
			Attr: attr,
		},
	}
}

// BlockSpec wraps the block and returns a spec.
func BlockSpec(block *Block) *Spec {
	return &Spec{
		Block: &Spec_BlockValue{
			BlockValue: block,
		},
	}
}

// BlockListSpec wraps the block list and returns a spec.
func BlockListSpec(blockList *BlockList) *Spec {
	return &Spec{
		Block: &Spec_BlockList{
			BlockList: blockList,
		},
	}
}

// BlockSetSpec wraps the block set and returns a spec.
func BlockSetSpec(blockSet *BlockSet) *Spec {
	return &Spec{
		Block: &Spec_BlockSet{
			BlockSet: blockSet,
		},
	}
}

// BlockMapSpec wraps the block map and returns a spec.
func BlockMapSpec(blockMap *BlockMap) *Spec {
	return &Spec{
		Block: &Spec_BlockMap{
			BlockMap: blockMap,
		},
	}
}

// DefaultSpec wraps the default and returns a spec.
func DefaultSpec(d *Default) *Spec {
	return &Spec{
		Block: &Spec_Default{
			Default: d,
		},
	}
}

// LiteralSpec wraps the literal and returns a spec.
func LiteralSpec(l *Literal) *Spec {
	return &Spec{
		Block: &Spec_Literal{
			Literal: l,
		},
	}
}

// NewObject returns a new object spec.
func NewObject(attrs map[string]*Spec) *Spec {
	return ObjectSpec(&Object{
		Attributes: attrs,
	})
}

// NewAttr returns a new attribute spec.
func NewAttr(name, attrType string, required bool) *Spec {
	return AttrSpec(&Attr{
		Name:     name,
		Type:     attrType,
		Required: required,
	})
}

// NewBlock returns a new block spec.
func NewBlock(name string, required bool, nested *Spec) *Spec {
	return BlockSpec(&Block{
		Name:     name,
		Required: required,
		Nested:   nested,
	})
}

// NewBlockList returns a new block list spec that has no limits.
func NewBlockList(name string, nested *Spec) *Spec {
	return NewBlockListLimited(name, 0, 0, nested)
}

// NewBlockListLimited returns a new block list spec that limits the number of
// blocks.
func NewBlockListLimited(name string, min, max uint64, nested *Spec) *Spec {
	return BlockListSpec(&BlockList{
		Name:     name,
		MinItems: min,
		MaxItems: max,
		Nested:   nested,
	})
}

// NewBlockSet returns a new block set spec that has no limits.
func NewBlockSet(name string, nested *Spec) *Spec {
	return NewBlockSetLimited(name, 0, 0, nested)
}

// NewBlockSetLimited returns a new block set spec that limits the number of
// blocks.
func NewBlockSetLimited(name string, min, max uint64, nested *Spec) *Spec {
	return BlockSetSpec(&BlockSet{
		Name:     name,
		MinItems: min,
		MaxItems: max,
		Nested:   nested,
	})
}

// NewBlockMap returns a new block map spec.
func NewBlockMap(name string, labels []string, nested *Spec) *Spec {
	return BlockMapSpec(&BlockMap{
		Name:   name,
		Labels: labels,
		Nested: nested,
	})
}

// NewLiteral returns a new literal spec.
func NewLiteral(value string) *Spec {
	return LiteralSpec(&Literal{
		Value: value,
	})
}

// NewDefault returns a new default spec.
func NewDefault(primary, defaultValue *Spec) *Spec {
	return DefaultSpec(&Default{
		Primary: primary,
		Default: defaultValue,
	})
}

// NewArray returns a new array spec.
func NewArray(values []*Spec) *Spec {
	return ArraySpec(&Array{
		Values: values,
	})
}
