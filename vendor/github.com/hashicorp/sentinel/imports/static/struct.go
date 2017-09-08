package static

import (
	"reflect"
	"strings"
	"sync"

	"github.com/hashicorp/sentinel-sdk/framework"
)

// Dynamic is an interface that allows structs to return dynamic values.
// This is called only if an exported field of the key name doesn't exist.
// If you want to force all values to go to the dynamic function, you can
// tag all fields with `sentinel:""`.
type Dynamic interface {
	SentinelGet(string) (interface{}, error)
}

// dynamicTyp is a reflect.Type for Dynamic.
var dynamicTyp = reflect.TypeOf((*Dynamic)(nil)).Elem()

type structNS struct {
	// value is a struct value
	value    reflect.Value
	original reflect.Value

	fieldMap map[string][]int
	ns       Dynamic
	once     sync.Once
}

// framework.Namespace impl.
func (m *structNS) Get(key string) (interface{}, error) {
	m.once.Do(m.init)

	// Lookup this field
	idx, ok := m.fieldMap[key]
	if !ok {
		// If our struct also implements Dynamic then we call that now
		if m.ns != nil {
			v, err := m.ns.SentinelGet(key)
			if err == nil {
				v = reflectReturn(v)
			}

			return v, err
		}

		return nil, nil
	}

	field := m.value.FieldByIndex(idx)
	if !field.IsValid() {
		return nil, nil
	}

	return reflectReturn(field.Interface()), nil
}

func (m *structNS) Map() (map[string]interface{}, error) {
	m.once.Do(m.init)

	keys := make([]string, 0, len(m.fieldMap))
	for k := range m.fieldMap {
		keys = append(keys, k)
	}

	return framework.MapFromKeys(m, keys)
}

func (m *structNS) init() {
	// Build our lookup table
	t := m.value.Type()
	m.fieldMap = make(map[string][]int, t.NumField())
	m.buildFieldMap(t, nil)

	// Check if the original implements dynamic
	if m.original.Type().Implements(dynamicTyp) {
		m.ns = m.original.Interface().(Dynamic)
	}
}

func (m *structNS) buildFieldMap(t reflect.Type, parent []int) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Ignore unexported fields
		if f.PkgPath != "" {
			continue
		}

		// Default key is the lowercased name
		key := underscore(f.Name)

		// Build the index for lookup
		fieldIndex := f.Index
		if parent != nil {
			fieldIndex = make([]int, 0, len(parent)+len(f.Index))
			fieldIndex = append(fieldIndex, parent...)
			fieldIndex = append(fieldIndex, f.Index...)
		}

		// If we have a tag, it is that value
		v, ok := f.Tag.Lookup("sentinel")
		if ok {
			// If empty string, it has no value
			if v == "" {
				continue
			}

			// If we have a comma, note the index
			idx := strings.IndexByte(v, ',')
			if idx > -1 {
				if f.Anonymous && v[idx+1:] == "squash" {
					m.buildFieldMap(f.Type, fieldIndex)
					continue
				}

				v = v[:idx]
			}

			key = v
		}

		// Setup the lookup
		m.fieldMap[key] = fieldIndex
	}
}
