package uuid

import (
	"github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var V4Func = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		uuid, err := uuid.NewRandom()
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}
		return cty.StringVal(uuid.String()), nil
	},
})

// V4 generates and returns a Type-4 UUID in the standard hexadecimal string
// format.
//
// This is not a "pure" function: it will generate a different result for each
// call.
func V4() (cty.Value, error) {
	return V4Func.Call(nil)
}
