package jsonhandles

import (
	"reflect"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// extendedTypes is a mapping of extended types to their extension function
	// TODO: the duplicates could be simplified by looking up the base type in the case of a pointer type in ConvertExt
	extendedTypes = map[reflect.Type]extendFunc{
		reflect.TypeOf(structs.Node{}):  nodeExt,
		reflect.TypeOf(&structs.Node{}): nodeExt,
	}
)

// nodeExt ensures the node is sanitized and adds the legacy field .Drain back to encoded Node objects
func nodeExt(v interface{}) interface{} {
	node := v.(*structs.Node).Sanitize()
	// transform to a struct with inlined Node fields plus the Drain field
	// - using defined type (not an alias!) EmbeddedNode gives us free conversion to a distinct type
	// - distinct type prevents this encoding extension from being called recursively/infinitely on the embedding
	// - pointers mean the conversion function doesn't have to make a copy during conversion
	type EmbeddedNode structs.Node
	return &struct {
		*EmbeddedNode
		Drain bool
	}{
		EmbeddedNode: (*EmbeddedNode)(node),
		Drain:        node != nil && node.DrainStrategy != nil,
	}
}
