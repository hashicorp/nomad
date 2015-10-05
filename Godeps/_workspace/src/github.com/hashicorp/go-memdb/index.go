package memdb

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
)

// Indexer is an interface used for defining indexes
type Indexer interface {
	// FromObject is used to extract an index value from an
	// object or to indicate that the index value is missing.
	FromObject(raw interface{}) (bool, []byte, error)

	// ExactFromArgs is used to build an exact index lookup
	// based on arguments
	FromArgs(args ...interface{}) ([]byte, error)
}

// PrefixIndexer can optionally be implemented for any
// indexes that support prefix based iteration. This may
// not apply to all indexes.
type PrefixIndexer interface {
	// PrefixFromArgs returns a prefix that should be used
	// for scanning based on the arguments
	PrefixFromArgs(args ...interface{}) ([]byte, error)
}

// StringFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type StringFieldIndex struct {
	Field     string
	Lowercase bool
}

func (s *StringFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Derefence the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	if s.Lowercase {
		val = strings.ToLower(val)
	}

	// Add the null character as a terminator
	val += "\x00"
	return true, []byte(val), nil
}

func (s *StringFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		arg = strings.ToLower(arg)
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *StringFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// UUIDFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field by treating
// it as a UUID. This is an optimization to using a StringFieldIndex
// as the UUID can be more compactly represented in byte form.
type UUIDFieldIndex struct {
	Field string
}

func (u *UUIDFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Derefence the pointer if any

	fv := v.FieldByName(u.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", u.Field, obj)
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	buf, err := u.parseString(val)
	return true, buf, err
}

func (u *UUIDFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	switch arg := args[0].(type) {
	case string:
		return u.parseString(arg)
	case []byte:
		if len(arg) != 16 {
			return nil, fmt.Errorf("byte slice must be 16 characters")
		}
		return arg, nil
	default:
		return nil,
			fmt.Errorf("argument must be a string or byte slice: %#v", args[0])
	}
}

func (u *UUIDFieldIndex) parseString(s string) ([]byte, error) {
	// Verify the length
	if len(s) != 36 {
		return nil, fmt.Errorf("UUID must be 36 characters")
	}

	// Decode each of the parts
	part1, err := hex.DecodeString(s[0:8])
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	part2, err := hex.DecodeString(s[9:13])
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	part3, err := hex.DecodeString(s[14:18])
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	part4, err := hex.DecodeString(s[19:23])
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	part5, err := hex.DecodeString(s[24:])
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	// Copy into a single buffer
	buf := make([]byte, 16)
	copy(buf[0:4], part1)
	copy(buf[4:6], part2)
	copy(buf[6:8], part3)
	copy(buf[8:10], part4)
	copy(buf[10:16], part5)
	return buf, nil
}

// CompoundIndex is used to build an index using multiple sub-indexes
// Prefix based iteration is supported as long as the appropriate prefix
// of indexers support it. All sub-indexers are only assumed to expect
// a single argument.
type CompoundIndex struct {
	Indexes []Indexer

	// AllowMissing results in an index based on only the indexers
	// that return data. If true, you may end up with 2/3 columns
	// indexed which might be useful for an index scan. Otherwise,
	// the CompoundIndex requires all indexers to be satisfied.
	AllowMissing bool
}

func (c *CompoundIndex) FromObject(raw interface{}) (bool, []byte, error) {
	var out []byte
	for i, idx := range c.Indexes {
		ok, val, err := idx.FromObject(raw)
		if err != nil {
			return false, nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		if !ok {
			if c.AllowMissing {
				break
			} else {
				return false, nil, nil
			}
		}
		out = append(out, val...)
	}
	return true, out, nil
}

func (c *CompoundIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != len(c.Indexes) {
		return nil, fmt.Errorf("less arguments than index fields")
	}
	return c.PrefixFromArgs(args...)
}

func (c *CompoundIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	var out []byte
	for i, arg := range args {
		val, err := c.Indexes[i].FromArgs(arg)
		if err != nil {
			return nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		out = append(out, val...)
	}
	return out, nil
}
