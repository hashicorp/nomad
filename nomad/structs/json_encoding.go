package structs

import (
	"reflect"

	"github.com/hashicorp/go-msgpack/codec"
)

func init() {
	registerExtension(reflect.TypeOf(Node{}), nodeExt)
}

func nodeExt(v interface{}) interface{} {
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

// BOILERPLATE GOES HERE

type extendFunc func(interface{}) interface{}

var (
	extendedTypes = map[reflect.Type]extendFunc{}
)

func registerExtension(tpe reflect.Type, ext extendFunc) {
	extendedTypes[tpe] = ext
}

type nomadJsonEncodingExtensions struct{}

// ConvertExt calls the registered conversions functions
func (n nomadJsonEncodingExtensions) ConvertExt(v interface{}) interface{} {
	if fn, ok := extendedTypes[reflect.TypeOf(v)]; ok {
		return fn(v)
	} else {
		return nil
	}
}

// UpdateExt is not used
func (n nomadJsonEncodingExtensions) UpdateExt(_ interface{}, _ interface{}) {}

func NomadJsonEncodingExtensions(h *codec.JsonHandle) *codec.JsonHandle {
	for tpe, _ := range extendedTypes {
		h.SetInterfaceExt(tpe, 1, nomadJsonEncodingExtensions{})
	}
	return h
}
