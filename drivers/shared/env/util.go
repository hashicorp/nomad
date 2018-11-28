package env

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

var (
	// ErrInvalidObjectPath is returned when a key cannot be converted into
	// a nested object path like "foo...bar", ".foo", or "foo."
	ErrInvalidObjectPath = errors.New("invalid object path")
)

type ErrKeyExists struct {
	msg string
}

func NewErrKeyExists(newKey, oldKey string) *ErrKeyExists {
	return &ErrKeyExists{
		msg: fmt.Sprintf(
			"cannot add key %q because %q already exists with a different type",
			newKey, oldKey,
		),
	}
}

func (e *ErrKeyExists) Error() string {
	return e.msg
}

// addNestedKey expands keys into their nested form:
//
//	k="foo.bar", v="quux" -> {"foo": {"bar": "quux"}}
//
// Existing keys are overwritten.
//
// If the key has dots but cannot be converted to a valid nested data structure
// (eg "foo...bar", "foo.", or non-object value exists for key), an error is
// returned.
func addNestedKey(dst map[string]interface{}, k, v string) error {
	// createdParent and Key capture the parent object of the first created
	// object and the first created object's key respectively. The cleanup
	// func deletes them to prevent side-effects when returning errors.
	var createdParent map[string]interface{}
	var createdKey string
	cleanup := func() {
		if createdParent != nil {
			delete(createdParent, createdKey)
		}
	}

	segments := strings.Split(k, ".")
	for _, newKey := range segments[:len(segments)-1] {
		if newKey == "" {
			// String either begins with a dot (.foo) or has at
			// least two consecutive dots (foo..bar); either way
			// it's an invalid object path.
			cleanup()
			return ErrInvalidObjectPath
		}

		var target map[string]interface{}
		if existingI, ok := dst[newKey]; ok {
			existing, ok := existingI.(map[string]interface{})
			if !ok {
				// Existing value is not a map, unable to support this key
				cleanup()
				return NewErrKeyExists(k, newKey)
			}
			target = existing
		} else {
			// Does not exist, create
			target = make(map[string]interface{})
			dst[newKey] = target

			// If this is the first created key, capture it for
			// cleanup if there is an error later.
			if createdParent == nil {
				createdParent = dst
				createdKey = newKey
			}
		}

		// Descend into new m
		dst = target
	}

	// See if the final segment is a valid key
	newKey := segments[len(segments)-1]
	if newKey == "" {
		// String ends in a dot
		cleanup()
		return ErrInvalidObjectPath
	}

	dst[newKey] = v
	return nil
}

// ctyify converts nested map[string]interfaces to a map[string]cty.Value. An
// error is returned if an unsupported type is encountered.
//
// Currently only strings, cty.Values, and nested maps are supported.
func ctyify(src map[string]interface{}) (map[string]cty.Value, error) {
	dst := make(map[string]cty.Value, len(src))

	for k, vI := range src {
		switch v := vI.(type) {
		case string:
			dst[k] = cty.StringVal(v)

		case cty.Value:
			dst[k] = v

		case map[string]interface{}:
			o, err := ctyify(v)
			if err != nil {
				return nil, err
			}
			dst[k] = cty.ObjectVal(o)

		default:
			return nil, fmt.Errorf("key %q has invalid type %T", k, v)
		}
	}

	return dst, nil
}
