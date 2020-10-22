package encoding

import (
	"net/url"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// URLEncodeFunc is a function that applies URL encoding to a given string.
var URLEncodeFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "str",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(url.QueryEscape(args[0].AsString())), nil
	},
})

// URLEncode applies URL encoding to a given string.
//
// This function identifies characters in the given string that would have a
// special meaning when included as a query string argument in a URL and
// escapes them using RFC 3986 "percent encoding".
//
// If the given string contains non-ASCII characters, these are first encoded as
// UTF-8 and then percent encoding is applied separately to each UTF-8 byte.
func URLEncode(str cty.Value) (cty.Value, error) {
	return URLEncodeFunc.Call([]cty.Value{str})
}
