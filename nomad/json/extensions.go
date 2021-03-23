package json

import (
	"reflect"

	"github.com/hashicorp/nomad/nomad/structs"
)

// init register all extensions used by the API HTTP server when encoding JSON
func init() {
	// TODO: this could be simplified by looking up the base type in the case of a pointer type
	registerExtension(reflect.TypeOf(structs.Node{}), nodeExt)
	registerExtension(reflect.TypeOf(&structs.Node{}), nodeExt)
}

// nodeExt adds the legacy field .Drain back to encoded Node objects
func nodeExt(v interface{}) interface{} {
	node := v.(*structs.Node)
	if node == nil {
		return nil
	}
	type NodeAlias structs.Node
	return &struct {
		*NodeAlias
		Drain bool
	}{
		NodeAlias: (*NodeAlias)(node),
		Drain:     node.DrainStrategy != nil,
	}
}
