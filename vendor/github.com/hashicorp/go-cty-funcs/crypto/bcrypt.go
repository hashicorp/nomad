package crypto

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
	"golang.org/x/crypto/bcrypt"
)

// BcryptFunc is a function that computes a hash of the given string using the
// Blowfish cipher.
var BcryptFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "str",
			Type: cty.String,
		},
	},
	VarParam: &function.Parameter{
		Name: "cost",
		Type: cty.Number,
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		defaultCost := 10

		if len(args) > 1 {
			var val int
			if err := gocty.FromCtyValue(args[1], &val); err != nil {
				return cty.UnknownVal(cty.String), err
			}
			defaultCost = val
		}

		if len(args) > 2 {
			return cty.UnknownVal(cty.String), fmt.Errorf("bcrypt() takes no more than two arguments")
		}

		input := args[0].AsString()
		out, err := bcrypt.GenerateFromPassword([]byte(input), defaultCost)
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("error occured generating password %s", err.Error())
		}

		return cty.StringVal(string(out)), nil
	},
})

// Bcrypt computes a hash of the given string using the Blowfish cipher,
// returning a string in the Modular Crypt Format usually expected in the
// shadow password file on many Unix systems.
func Bcrypt(str cty.Value, cost ...cty.Value) (cty.Value, error) {
	args := make([]cty.Value, len(cost)+1)
	args[0] = str
	copy(args[1:], cost)
	return BcryptFunc.Call(args)
}
