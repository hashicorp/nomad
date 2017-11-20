package static

import (
	"reflect"
)

var interfaceTyp = reflect.TypeOf((*[]interface{})(nil)).Elem()

// reflectReturn can be called on a resulting value to determine what
// further recursion can be done to the right namespace.
func reflectReturn(raw interface{}) interface{} {
	// If the value is a map, recurse further by returning a namespace
	if m, ok := raw.(map[string]interface{}); ok {
		return &mapNS{objects: m}
	}

	// Use reflection to determine the kind of value this is
	v := reflectValue(reflect.ValueOf(raw))
	if !v.IsValid() {
		return nil
	}

	return v.Interface()
}

func reflectValue(original reflect.Value) reflect.Value {
	v := original
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// If after all the indirection we're left with an invalid value, do nothing
	if !v.IsValid() {
		return original
	}

	switch v.Kind() {
	case reflect.Struct:
		// If the value is a struct, setup a struct lookup
		return reflect.ValueOf(&structNS{
			value:    v,
			original: original,
		})

	case reflect.Slice:
		// For slices, we need to check each value
		return reflectSlice(original, v)
	}

	return original
}

func reflectSlice(original, v reflect.Value) reflect.Value {
	new := reflect.MakeSlice(interfaceTyp, v.Len(), v.Len())
	for i := 0; i < v.Len(); i++ {
		new.Index(i).Set(reflectValue(v.Index(i)))
	}

	return new
}
