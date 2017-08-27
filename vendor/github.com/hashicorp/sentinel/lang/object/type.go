package object

import (
	"strconv"
)

// Type is the type of an Object.
type Type int

// The list of types
const (
	ILLEGAL Type = iota

	UNDEFINED
	NULL

	comparable_beg // the types between this and end are comparable
	BOOL

	ordered_beg // the types between this and end are ordered (support rel ops)
	INT
	FLOAT
	STRING
	ordered_end
	comparable_end

	collection_beg // the types here are collections
	LIST
	MAP
	collection_end

	RULE
	FUNC
	EXTERNAL
	RUNTIME
)

// String values for types
var types = [...]string{
	ILLEGAL: "illegal",

	UNDEFINED: "undefined",
	NULL:      "null",

	BOOL: "bool",

	INT:    "int",
	FLOAT:  "float",
	STRING: "string",

	LIST: "list",
	MAP:  "map",

	RULE:     "rule",
	FUNC:     "func",
	EXTERNAL: "external",
	RUNTIME:  "runtime",
}

// String returns the string corresponding to the type t. This is meant
// to be human-friendly for valid type values and should be used for
// error messages.
func (t Type) String() string {
	s := ""
	if 0 <= t && t < Type(len(types)) {
		s = types[t]
	}
	if s == "" {
		s = "type(" + strconv.Itoa(int(t)) + ")"
	}
	return s
}

// IsComparable returns true for types that are comparable (==, !=, etc.)
func (t Type) IsComparable() bool { return comparable_beg < t && t < comparable_end }

// IsOrdered returns true for types that are ordered (<, >, etc.)
func (t Type) IsOrdered() bool { return ordered_beg < t && t < ordered_end }

// IsCollection returns true for types that are collections (support contains, in)
func (t Type) IsCollection() bool { return collection_beg < t && t < collection_end }
