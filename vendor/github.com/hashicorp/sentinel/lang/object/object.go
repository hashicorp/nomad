// Package object contains interfaces and structures to represent
// object values within Sentinel.
//
// Runtime implementations may choose not to use these representations.
// The purpose for a standard set of representations of object values
// is to have a single share-able API for optimizers, semantic passes,
// package implementations, etc. It doesn't need to be used for actual
// execution.
package object

import (
	"fmt"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

// Object is the value of an entity.
type Object interface {
	Type() Type     // The type of this value
	String() string // Human-friendly representation for debugging

	object()
}

type (
	// UndefinedObj represents an undefined value.
	UndefinedObj struct {
		Pos []token.Pos // len(Pos) > 0
	}

	// nullObj represents null. This isn't exported because it is a
	// singleton that should be referenced with Null
	nullObj struct{}

	// boolObj represents a boolean value. This value should NOT be
	// created directly. Instead, the singleton value should be created
	// with the Bool function.
	BoolObj struct {
		Value bool
	}

	// IntObj represents an integer value.
	IntObj struct {
		Value int64
	}

	// FloatObj represents a float value.
	FloatObj struct {
		Value float64
	}

	// StringObj represents a string value.
	StringObj struct {
		Value string
	}

	// ListObj represents a list of values.
	ListObj struct {
		Elts []Object
	}

	// MapObj represents a key/value mapping.
	MapObj struct {
		Elts []KeyedObj
	}

	// RuleObj represents a rule.
	RuleObj struct {
		Expr  ast.Expr // Expr is the un-evaluated expression
		Scope *Scope   // Scope to evaluate the rule in
		Eval  bool     // Eval is true if evaluated
		Value Object   // Value is the set value once evaluated
	}

	// KeyedObj represents a key/value pair. This doens't actual implement
	// Object.
	KeyedObj struct {
		Key   Object
		Value Object
	}

	// FuncObj represents a function
	FuncObj struct {
		Params []*ast.Ident
		Body   []ast.Stmt
		Scope  *Scope // Scope to evaluate the func in
	}

	// ExternalObj represents external keyed data that is loaded on-demand.
	ExternalObj struct {
		External External
	}

	// RuntimeObj represents runtime-specific data. This is unsafe to use
	// unless you're sure that the runtime will be able to handle this data.
	// Please reference the runtime documentation for more information.
	RuntimeObj struct {
		Value interface{}
	}
)

func (o *UndefinedObj) Type() Type { return UNDEFINED }
func (o *nullObj) Type() Type      { return NULL }
func (o *BoolObj) Type() Type      { return BOOL }
func (o *IntObj) Type() Type       { return INT }
func (o *FloatObj) Type() Type     { return FLOAT }
func (o *StringObj) Type() Type    { return STRING }
func (o *ListObj) Type() Type      { return LIST }
func (o *MapObj) Type() Type       { return MAP }
func (o *RuleObj) Type() Type      { return RULE }
func (o *FuncObj) Type() Type      { return FUNC }
func (o *ExternalObj) Type() Type  { return EXTERNAL }
func (o *RuntimeObj) Type() Type   { return RUNTIME }

func (o *UndefinedObj) String() string { return "undefined" }
func (o *nullObj) String() string      { return "null" }
func (o *BoolObj) String() string      { return fmt.Sprintf("%v", o.Value) }
func (o *IntObj) String() string       { return fmt.Sprintf("%d", o.Value) }
func (o *FloatObj) String() string     { return fmt.Sprintf("%f", o.Value) }
func (o *StringObj) String() string    { return fmt.Sprintf("%q", o.Value) }
func (o *ListObj) String() string      { return fmt.Sprintf("%s", o.Elts) }
func (o *MapObj) String() string       { return fmt.Sprintf("%s", o.Elts) }
func (o *RuleObj) String() string {
	// TODO: use printer here once we have it

	if !o.Eval {
		return fmt.Sprintf("un-evaluated rule: %#v", o.Expr)
	}

	return fmt.Sprintf("evaluated rule. result: %v, result: %s", o.Value, o.Expr)
}
func (o *FuncObj) String() string     { return "func" }
func (o *ExternalObj) String() string { return "external" }
func (o *RuntimeObj) String() string  { return "runtime" }

func (o *UndefinedObj) object() {}
func (o *nullObj) object()      {}
func (o *BoolObj) object()      {}
func (o *IntObj) object()       {}
func (o *FloatObj) object()     {}
func (o *StringObj) object()    {}
func (o *ListObj) object()      {}
func (o *MapObj) object()       {}
func (o *RuleObj) object()      {}
func (o *FuncObj) object()      {}
func (o *ExternalObj) object()  {}
func (o *RuntimeObj) object()   {}
