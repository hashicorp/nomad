// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"reflect"

	"github.com/hashicorp/go-msgpack/codec"
)

// extendFunc is a mapping from one struct to another, to change the shape of the encoded JSON
type extendFunc func(interface{}) interface{}

// nomadJsonEncodingExtensions is a catch-all go-msgpack extension
// it looks up the types in the list of registered extension functions and applies it
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

// UpdateExt is required by go-msgpack, but not used by us
func (n nomadJsonEncodingExtensions) UpdateExt(_ interface{}, _ interface{}) {}

// NomadJsonEncodingExtensions registers all extension functions against the
// provided JsonHandle.
// It should be called on any JsonHandle which is used by the API HTTP server.
func NomadJsonEncodingExtensions(h *codec.JsonHandle) *codec.JsonHandle {
	for tpe := range extendedTypes {
		h.SetInterfaceExt(tpe, 1, nomadJsonEncodingExtensions{})
	}
	return h
}
