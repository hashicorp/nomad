package memdb

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
)

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
	return []byte(arg), nil
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
