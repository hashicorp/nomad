package helper

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/hcl/ast"
)

// CompareMapStringString returns true if the maps are equivalent. A nil and
// empty map are considered not equal.
func CompareMapStringString(a, b map[string]string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}
		if v != v2 {
			return false
		}
	}

	// Already compared all known values in a so only test that keys from b
	// exist in a
	for k := range b {
		if _, ok := a[k]; !ok {
			return false
		}
	}

	return true
}

// CompareSliceSetString returns true if the slices contain the same strings.
// Order is ignored. The slice may be copied but is never altered. The slice is
// assumed to be a set. Multiple instances of an entry are treated the same as
// a single instance.
func CompareSliceSetString(a, b []string) bool {
	n := len(a)
	if n != len(b) {
		return false
	}

	// Copy a into a map and compare b against it
	amap := make(map[string]struct{}, n)
	for i := range a {
		amap[a[i]] = struct{}{}
	}

	for i := range b {
		if _, ok := amap[b[i]]; !ok {
			return false
		}
	}

	return true
}

// CopyMapStringSliceString copies a map of strings to string slices such as
// http.Header
func CopyMapStringSliceString(m map[string][]string) map[string][]string {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[string][]string, l)
	for k, v := range m {
		c[k] = CopySliceString(v)
	}
	return c
}

func CopyMapStringInt(m map[string]int) map[string]int {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[string]int, l)
	for k, v := range m {
		c[k] = v
	}
	return c
}

func CopyMapStringFloat64(m map[string]float64) map[string]float64 {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[string]float64, l)
	for k, v := range m {
		c[k] = v
	}
	return c
}

// Helpers for copying generic structures.
func CopyMapStringString(m map[string]string) map[string]string {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[string]string, l)
	for k, v := range m {
		c[k] = v
	}
	return c
}

func CopySliceString(s []string) []string {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]string, l)
	for i, v := range s {
		c[i] = v
	}
	return c
}

// boolToPtr returns the pointer to a boolean
func BoolToPtr(b bool) *bool {
	return &b
}

// Float64ToPtr returns the pointer to an float64
func Float64ToPtr(f float64) *float64 {
	return &f
}

// IntToPtr returns the pointer to an int
func IntToPtr(i int) *int {
	return &i
}

// Int8ToPtr returns the pointer to an int8
func Int8ToPtr(i int8) *int8 {
	return &i
}

// Int64ToPtr returns the pointer to an int
func Int64ToPtr(i int64) *int64 {
	return &i
}

// StringToPtr returns the pointer to a string
func StringToPtr(str string) *string {
	return &str
}

// TimeToPtr returns the pointer to a time stamp
func TimeToPtr(t time.Duration) *time.Duration {
	return &t
}

// Uint64ToPtr returns the pointer to an uint64
func Uint64ToPtr(u uint64) *uint64 {
	return &u
}

func SliceSetDisjoint(first, second []string) (bool, []string) {
	contained := make(map[string]struct{}, len(first))
	for _, k := range first {
		contained[k] = struct{}{}
	}

	offending := make(map[string]struct{})
	for _, k := range second {
		if _, ok := contained[k]; ok {
			offending[k] = struct{}{}
		}
	}

	if len(offending) == 0 {
		return true, nil
	}

	flattened := make([]string, 0, len(offending))
	for k := range offending {
		flattened = append(flattened, k)
	}
	return false, flattened
}

func CheckHCLKeys(node ast.Node, valid []string) error {
	var list *ast.ObjectList
	switch n := node.(type) {
	case *ast.ObjectList:
		list = n
	case *ast.ObjectType:
		list = n.List
	default:
		return fmt.Errorf("cannot check HCL keys of type %T", n)
	}

	validMap := make(map[string]struct{}, len(valid))
	for _, v := range valid {
		validMap[v] = struct{}{}
	}

	var result error
	for _, item := range list.Items {
		key := item.Keys[0].Token.Value().(string)
		if _, ok := validMap[key]; !ok {
			result = multierror.Append(result, fmt.Errorf(
				"invalid key: %s", key))
		}
	}

	return result
}

// UnusedKeys returns a pretty-printed error if any `hcl:",unusedKeys"` is not empty
func UnusedKeys(obj interface{}) error {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	return unusedKeysImpl([]string{}, val)
}

func unusedKeysImpl(path []string, val reflect.Value) error {
	stype := val.Type()
	for i := 0; i < stype.NumField(); i++ {
		ftype := stype.Field(i)
		fval := val.Field(i)
		tags := strings.Split(ftype.Tag.Get("hcl"), ",")
		name := tags[0]
		tags = tags[1:]

		if fval.Kind() == reflect.Ptr {
			fval = reflect.Indirect(fval)
		}

		// struct? recurse. Add the struct's key to the path
		if fval.Kind() == reflect.Struct {
			err := unusedKeysImpl(append([]string{name}, path...), fval)
			if err != nil {
				return err
			}
			continue
		}

		// Search the hcl tags for "unusedKeys"
		unusedKeys := false
		for _, p := range tags {
			if p == "unusedKeys" {
				unusedKeys = true
				break
			}
		}

		if unusedKeys {
			ks, ok := fval.Interface().([]string)
			if ok && len(ks) != 0 {
				ps := ""
				if len(path) > 0 {
					ps = strings.Join(path, ".") + " "
				}
				return fmt.Errorf("%sunexpected keys %s",
					ps,
					strings.Join(ks, ", "))
			}
		}
	}
	return nil
}
