package framework

import (
	"reflect"
)

// flattenMapValues takes a map[string]interface{}, finds any values that
// implement Map, and calls it. It does this recursively.
//
// This function enables namespaces to return other namespaces as values in
// their Map functions. The framework then automatically handles turning
// all of these into maps.
//
// In the future, we may support "thunk" results that are lighter weight.
// For now, we flatten everything.
func flattenMapValues(mv reflect.Value) (interface{}, error) {
	for _, k := range mv.MapKeys() {
		v := mv.MapIndex(k)
		for v.Kind() == reflect.Interface {
			v = v.Elem()
		}

		var result interface{}
		var err error
		if v.Kind() == reflect.Map {
			result, err = flattenMapValues(v)
		} else if m, ok := v.Interface().(Map); ok {
			result, err = m.Map()
		} else {
			// Not a map type, iterate again
			continue
		}

		// Run flatten again so we can recurse
		if err == nil {
			result, err = flattenMapValues(reflect.ValueOf(result))
		}

		if err != nil {
			return nil, err
		}

		mv.SetMapIndex(k, reflect.ValueOf(result))
	}

	return mv.Interface(), nil
}
