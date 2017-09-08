package eval

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/sentinel/lang/object"
)

// Universe contains the pre-declared identifiers that are part of the
// language specification. This should be the parent of any scope used
// for interpretation.
//
// This should not be modified ever.
var Universe = &object.Scope{
	Objects: map[string]object.Object{
		"true":  object.True,
		"false": object.False,
		"null":  object.Null,

		// Builtins
		"append": object.ExternalFunc(builtin_append),
		"keys":   object.ExternalFunc(builtin_keys),
		"values": object.ExternalFunc(builtin_values),
		"length": object.ExternalFunc(builtin_length),
		"int":    object.ExternalFunc(builtin_int),
		"float":  object.ExternalFunc(builtin_float),
		"string": object.ExternalFunc(builtin_string),
	},
}

// UndefinedName is the global constant for the "undefined" identifier.
const UndefinedName = "undefined"

//-------------------------------------------------------------------
// Builtin Functions
//
// These all have minimal unit tests because the lang/spec tests cover
// the various cases of the built-in functions..

// length
func builtin_length(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	switch x := args[0].(type) {
	case *object.StringObj:
		return len(x.Value), nil

	case *object.ListObj:
		return len(x.Elts), nil

	case *object.MapObj:
		return len(x.Elts), nil

	case *object.UndefinedObj:
		return x, nil

	default:
		return nil, fmt.Errorf(
			"length can only be called with strings, lists, or maps, got %q",
			args[0].Type())
	}
}

// append
func builtin_append(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 2, 2); err != nil {
		return nil, err
	}

	x, ok := args[0].(*object.ListObj)
	if !ok {
		return nil, fmt.Errorf(
			"append first argument can only be called with lists, got %q",
			args[0].Type())
	}

	x.Elts = append(x.Elts, args[1])
	return x, nil
}

// keys
func builtin_keys(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	x, ok := args[0].(*object.MapObj)
	if !ok {
		if x, ok := args[0].(*object.UndefinedObj); ok {
			return x, nil
		}

		return nil, fmt.Errorf(
			"keys first argument can only be called with maps, got %q",
			args[0].Type())
	}

	result := make([]object.Object, len(x.Elts))
	for i, elt := range x.Elts {
		result[i] = elt.Key
	}

	return &object.ListObj{Elts: result}, nil
}

// values
func builtin_values(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	x, ok := args[0].(*object.MapObj)
	if !ok {
		if x, ok := args[0].(*object.UndefinedObj); ok {
			return x, nil
		}

		return nil, fmt.Errorf(
			"values first argument can only be called with maps, got %q",
			args[0].Type())
	}

	result := make([]object.Object, len(x.Elts))
	for i, elt := range x.Elts {
		result[i] = elt.Value
	}

	return &object.ListObj{Elts: result}, nil
}

// int
func builtin_int(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	switch x := args[0].(type) {
	case *object.IntObj:
		return x, nil

	case *object.StringObj:
		v, err := strconv.ParseInt(x.Value, 0, 64)
		if err != nil {
			return nil, err
		}

		return &object.IntObj{Value: v}, nil

	case *object.FloatObj:
		return &object.IntObj{
			Value: int64(math.Floor(x.Value)),
		}, nil

	case *object.UndefinedObj:
		return x, nil

	default:
		return &object.UndefinedObj{}, nil
	}
}

// float
func builtin_float(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	switch x := args[0].(type) {
	case *object.IntObj:
		return &object.FloatObj{Value: float64(x.Value)}, nil

	case *object.StringObj:
		v, err := strconv.ParseFloat(x.Value, 64)
		if err != nil {
			return nil, err
		}

		return &object.FloatObj{Value: v}, nil

	case *object.FloatObj:
		return x, nil

	case *object.UndefinedObj:
		return x, nil

	default:
		return &object.UndefinedObj{}, nil
	}
}

// string
func builtin_string(args []object.Object) (interface{}, error) {
	if err := builtinArgCount(args, 1, 1); err != nil {
		return nil, err
	}

	switch x := args[0].(type) {
	case *object.IntObj:
		return &object.StringObj{Value: strconv.FormatInt(x.Value, 10)}, nil

	case *object.StringObj:
		return x, nil

	case *object.FloatObj:
		return &object.StringObj{
			Value: strconv.FormatFloat(x.Value, 'f', -1, 64),
		}, nil

	case *object.UndefinedObj:
		return x, nil

	default:
		return &object.UndefinedObj{}, nil
	}
}

func builtinArgCount(args []object.Object, min, max int) error {
	// Exact case
	if min == max {
		if len(args) != min {
			return fmt.Errorf(
				"invalid number of arguments, expected %d, got %d",
				len(args), min)
		}

		return nil
	}

	return nil
}
