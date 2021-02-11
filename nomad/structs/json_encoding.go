package structs

import (
	"reflect"

	"github.com/hashicorp/go-msgpack/codec"
)

// Special encoding for structs.Node, to perform the following:
// 1. provide backwards compatibility for the following fields:
//   * Node.Drain
type nodeExt struct{}

// ConvertExt converts a structs.Node to a struct with the extra field, Drain
func (n nodeExt) ConvertExt(v interface{}) interface{} {
	node := v.(*Node)
	if node == nil {
		return nil
	}
	type NodeAlias Node
	return &struct {
		*NodeAlias
		Drain bool
	}{
		NodeAlias: (*NodeAlias)(node),
		Drain:     node.DrainStrategy != nil,
	}
}

// UpdateExt is not used
func (n nodeExt) UpdateExt(_ interface{}, _ interface{}) {}

func RegisterJSONEncodingExtensions(h *codec.JsonHandle) *codec.JsonHandle {
	h.SetInterfaceExt(reflect.TypeOf(Node{}), 1, nodeExt{})
	return h
}
