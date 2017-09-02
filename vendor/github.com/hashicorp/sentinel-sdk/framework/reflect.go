package framework

import (
	"reflect"
)

// mapTyp is a reflect.Type for Map.
var mapTyp = reflect.TypeOf((*Map)(nil)).Elem()

// Reflect takes a value and uses reflection to traverse the value, finding
// any further namespaces that need to be converted to types that can be
// sent across the plugin barrier.
//
// Currently, this means flattening them all to maps. In the future, we intend
// to support "thunks" to allow efficiently transferring this data without
// having to flatten it all.
func (m *Import) reflect(value interface{}) (interface{}, error) {
	v, err := m.reflectValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}

	if !v.IsValid() {
		return nil, nil
	}

	return v.Interface(), nil
}

func (m *Import) reflectValue(v reflect.Value) (reflect.Value, error) {
	// If the value isn't valid, return right away
	if !v.IsValid() {
		return v, nil
	}

	// Unwrap the interface wrappers
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	// Determine if we have a nil pointer. This will turn a typed nil
	// into a plain nil so that we can turn it into an undefined value
	// properly.
	ptr := v
	for ptr.Kind() == reflect.Ptr {
		ptr = ptr.Elem()
	}
	if !ptr.IsValid() {
		return ptr, nil
	}

	// If the value implements Map, then we call that and use the map
	// value as the actual thing to look at.
	if v.Type().Implements(mapTyp) {
		m, err := v.Interface().(Map).Map()
		if err != nil {
			return v, err
		}

		v = reflect.ValueOf(m)
	}

	switch v.Kind() {
	case reflect.Map:
		return m.reflectMap(v)

	case reflect.Slice:
		return m.reflectSlice(v)

	default:
		return v, nil
	}
}

func (m *Import) reflectMap(mv reflect.Value) (reflect.Value, error) {
	for _, k := range mv.MapKeys() {
		v, err := m.reflectValue(mv.MapIndex(k))
		if err != nil {
			return mv, err
		}

		mv.SetMapIndex(k, v)
	}

	return mv, nil
}

func (m *Import) reflectSlice(v reflect.Value) (reflect.Value, error) {
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		newElem, err := m.reflectValue(elem)
		if err != nil {
			return v, err
		}

		elem.Set(newElem)
	}

	return v, nil
}
