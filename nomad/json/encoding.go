package json

import (
	"reflect"

	"github.com/hashicorp/go-msgpack/codec"
)

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
		// shouldn't get here, but returning v will probably result in an infinite loop
		// return nil and erase this field
		return nil
	}
}

// UpdateExt is not used
func (n nomadJsonEncodingExtensions) UpdateExt(_ interface{}, _ interface{}) {}

func NomadJsonEncodingExtensions(h *codec.JsonHandle) *codec.JsonHandle {
	for tpe := range extendedTypes {
		h.SetInterfaceExt(tpe, 1, nomadJsonEncodingExtensions{})
	}
	return h
}
