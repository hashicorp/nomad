package memdb

import (
	"fmt"
	"reflect"
	"strings"
)

// StringFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
func StringFieldIndex(field string, lowercase bool) IndexerFunc {
	return func(obj interface{}) (bool, []byte, error) {
		v := reflect.ValueOf(obj)
		v = reflect.Indirect(v) // Derefence the pointer if any

		fv := v.FieldByName(field)
		if !fv.IsValid() {
			return false, nil,
				fmt.Errorf("field '%s' for %#v is invalid", field, obj)
		}

		val := fv.String()
		if val == "" {
			return false, nil, nil
		}

		if lowercase {
			val = strings.ToLower(val)
		}
		return true, []byte(val), nil
	}
}
