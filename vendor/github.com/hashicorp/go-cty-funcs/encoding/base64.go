package encoding

import (
	"encoding/base64"
	"fmt"
	"log"
	"unicode/utf8"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Base64DecodeFunc is a function that decodes a string containing a base64 sequence.
var Base64DecodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "str",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		s := args[0].AsString()
		sDec, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("failed to decode base64 data '%s'", s)
		}
		if !utf8.Valid([]byte(sDec)) {
			log.Printf("[DEBUG] the result of decoding the the provided string is not valid UTF-8: %s", sDec)
			return cty.UnknownVal(cty.String), fmt.Errorf("the result of decoding the the provided string is not valid UTF-8")
		}
		return cty.StringVal(string(sDec)), nil
	},
})

// Base64EncodeFunc is a function that encodes a string to a base64 sequence.
var Base64EncodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "str",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(base64.StdEncoding.EncodeToString([]byte(args[0].AsString()))), nil
	},
})

// Base64Decode decodes a string containing a base64 sequence.
//
// Terraform uses the "standard" Base64 alphabet as defined in RFC 4648 section 4.
//
// Strings in the Terraform language are sequences of unicode characters rather
// than bytes, so this function will also interpret the resulting bytes as
// UTF-8. If the bytes after Base64 decoding are _not_ valid UTF-8, this function
// produces an error.
func Base64Decode(str cty.Value) (cty.Value, error) {
	return Base64DecodeFunc.Call([]cty.Value{str})
}

// Base64Encode applies Base64 encoding to a string.
//
// Terraform uses the "standard" Base64 alphabet as defined in RFC 4648 section 4.
//
// Strings in the Terraform language are sequences of unicode characters rather
// than bytes, so this function will first encode the characters from the string
// as UTF-8, and then apply Base64 encoding to the result.
func Base64Encode(str cty.Value) (cty.Value, error) {
	return Base64EncodeFunc.Call([]cty.Value{str})
}
