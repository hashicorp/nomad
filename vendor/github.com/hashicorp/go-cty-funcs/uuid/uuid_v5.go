package uuid

import (
	"fmt"

	uuidv5 "github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var V5Func = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "namespace",
			Type: cty.String,
		},
		{
			Name: "name",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		var namespace uuidv5.UUID
		switch {
		case args[0].AsString() == "dns":
			namespace = uuidv5.NameSpaceDNS
		case args[0].AsString() == "url":
			namespace = uuidv5.NameSpaceURL
		case args[0].AsString() == "oid":
			namespace = uuidv5.NameSpaceOID
		case args[0].AsString() == "x500":
			namespace = uuidv5.NameSpaceX500
		default:
			if namespace, err = uuidv5.Parse(args[0].AsString()); err != nil {
				return cty.UnknownVal(cty.String), fmt.Errorf("uuidv5() doesn't support namespace %s (%v)", args[0].AsString(), err)
			}
		}
		val := args[1].AsString()
		return cty.StringVal(uuidv5.NewSHA1(namespace, []byte(val)).String()), nil
	},
})

// V5 generates and returns a Type-5 UUID in the standard hexadecimal
// string format.
//
// This is not a "pure" function: it will generate a different result for each
// call.
func V5(namespace cty.Value, name cty.Value) (cty.Value, error) {
	return V5Func.Call([]cty.Value{namespace, name})
}
