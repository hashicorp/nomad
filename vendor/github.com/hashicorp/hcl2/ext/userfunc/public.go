package userfunc

import (
	"github.com/hashicorp/hcl2/hcl"
	"github.com/zclconf/go-cty/cty/function"
)

// A ContextFunc is a callback used to produce the base EvalContext for
// running a particular set of functions.
//
// This is a function rather than an EvalContext directly to allow functions
// to be decoded before their context is complete. This will be true, for
// example, for applications that wish to allow functions to refer to themselves.
//
// The simplest use of a ContextFunc is to give user functions access to the
// same global variables and functions available elsewhere in an application's
// configuration language, but more complex applications may use different
// contexts to support lexical scoping depending on where in a configuration
// structure a function declaration is found, etc.
type ContextFunc func() *hcl.EvalContext

// DecodeUserFunctions looks for blocks of the given type in the given body
// and, for each one found, interprets it as a custom function definition.
//
// On success, the result is a mapping of function names to implementations,
// along with a new body that represents the remaining content of the given
// body which can be used for further processing.
//
// The result expression of each function is parsed during decoding but not
// evaluated until the function is called.
//
// If the given ContextFunc is non-nil, it will be called to obtain the
// context in which the function result expressions will be evaluated. If nil,
// or if it returns nil, the result expression will have access only to
// variables named after the declared parameters. A non-nil context turns
// the returned functions into closures, bound to the given context.
//
// If the returned diagnostics set has errors then the function map and
// remain body may be nil or incomplete.
func DecodeUserFunctions(body hcl.Body, blockType string, context ContextFunc) (funcs map[string]function.Function, remain hcl.Body, diags hcl.Diagnostics) {
	return decodeUserFunctions(body, blockType, context)
}
