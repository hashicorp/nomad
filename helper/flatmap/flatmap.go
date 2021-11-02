package flatmap

import (
	"fmt"
	"reflect"
	"strconv"
)

// Flatten takes an object and returns a flat map of the object. The keys of the
// map is the path of the field names until a primitive field is reached and the
// value is a string representation of the terminal field.
func Flatten(obj interface{}, filter []string, primitiveOnly bool) map[string]string {
	flat := make(map[string]string)
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		return nil
	}

	flatten("", v, primitiveOnly, false, flat, PrefixMakers{})
	for _, f := range filter {
		delete(flat, f)
	}
	return flat
}

// FlattenDotPrefix does the same as Flatten, but generates struct-style
// dot access prefixes even for maps
func FlattenDotPrefix(obj interface{}, filter []string, primitiveOnly bool) map[string]string {
	flat := make(map[string]string)
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		return nil
	}

	flatten("", v, primitiveOnly, false, flat, PrefixMakers{
		forMap: getSubPrefixStruct,
	})
	for _, f := range filter {
		delete(flat, f)
	}
	return flat
}

type PrefixMakers struct {
	forStruct, forMap func(string, string) string
	forArray          func(string, int) string
}

// flatten recursively calls itself to create a flatmap representation of the
// passed value. The results are stored into the output map and the keys are
// the fields prepended with the passed prefix.
// XXX: A current restriction is that maps only support string keys.
func flatten(prefix string, v reflect.Value, primitiveOnly, enteredStruct bool, output map[string]string, prefixMakers PrefixMakers) {
	if prefixMakers.forMap == nil {
		prefixMakers.forMap = getSubPrefixMap
	}
	if prefixMakers.forStruct == nil {
		prefixMakers.forStruct = getSubPrefixStruct
	}
	if prefixMakers.forArray == nil {
		prefixMakers.forArray = getSubPrefixArray
	}

	switch v.Kind() {
	case reflect.Bool:
		output[prefix] = fmt.Sprintf("%v", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		output[prefix] = fmt.Sprintf("%v", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		output[prefix] = fmt.Sprintf("%v", v.Uint())
	case reflect.Float32, reflect.Float64:
		output[prefix] = fmt.Sprintf("%v", v.Float())
	case reflect.Complex64, reflect.Complex128:
		output[prefix] = fmt.Sprintf("%v", v.Complex())
	case reflect.String:
		output[prefix] = fmt.Sprintf("%v", v.String())
	case reflect.Invalid:
		output[prefix] = "nil"
	case reflect.Ptr:
		if primitiveOnly && enteredStruct {
			return
		}

		e := v.Elem()
		if !e.IsValid() {
			output[prefix] = "nil"
		}
		flatten(prefix, e, primitiveOnly, enteredStruct, output, prefixMakers)
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if k.Kind() == reflect.Interface {
				k = k.Elem()
			}

			if k.Kind() != reflect.String {
				panic(fmt.Sprintf("%q: map key is not string: %s", prefix, k))
			}

			flatten(prefixMakers.forMap(prefix, k.String()), v.MapIndex(k), primitiveOnly, enteredStruct, output, prefixMakers)
		}
	case reflect.Struct:
		if primitiveOnly && enteredStruct {
			return
		}
		enteredStruct = true

		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Name
			val := v.Field(i)
			if val.Kind() == reflect.Interface && !val.IsNil() {
				val = val.Elem()
			}

			flatten(prefixMakers.forStruct(prefix, name), val, primitiveOnly, enteredStruct, output, prefixMakers)
		}
	case reflect.Interface:
		if primitiveOnly {
			return
		}

		e := v.Elem()
		if !e.IsValid() {
			output[prefix] = "nil"
			return
		}
		flatten(prefix, e, primitiveOnly, enteredStruct, output, prefixMakers)
	case reflect.Array, reflect.Slice:
		if primitiveOnly {
			return
		}

		if v.Kind() == reflect.Slice && v.IsNil() {
			output[prefix] = "nil"
			return
		}
		for i := 0; i < v.Len(); i++ {
			flatten(prefixMakers.forArray(prefix, i), v.Index(i), primitiveOnly, enteredStruct, output, prefixMakers)
		}
	default:
		panic(fmt.Sprintf("prefix %q; unsupported type %v", prefix, v.Kind()))
	}
}

// getSubPrefixStruct takes the current prefix and the next subfield and returns an
// appropriate prefix for a struct member.
func getSubPrefixStruct(curPrefix, subField string) string {
	if curPrefix != "" {
		return fmt.Sprintf("%s.%s", curPrefix, subField)
	}
	return subField
}

// getSubPrefixMap takes the current prefix and the next subfield and returns an
// appropriate prefix for a map field.
func getSubPrefixMap(curPrefix, subField string) string {
	if curPrefix != "" {
		return fmt.Sprintf("%s[%s]", curPrefix, subField)
	}
	return subField
}

// getSubPrefixArray takes the current prefix and the next subfield and
// returns an appropriate prefix for an array element.
func getSubPrefixArray(curPrefix string, index int) string {
	return getSubPrefixMap(curPrefix, strconv.Itoa(index))
}
