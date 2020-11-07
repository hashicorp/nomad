package cidr

import (
	"fmt"
	"net"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// NetmaskFunc is a function that converts an IPv4 address prefix given in CIDR
// notation into a subnet mask address.
var NetmaskFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "prefix",
			Type: cty.String,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		_, network, err := net.ParseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("invalid CIDR expression: %s", err)
		}

		return cty.StringVal(net.IP(network.Mask).String()), nil
	},
})

// Netmask converts an IPv4 address prefix given in CIDR notation into a subnet mask address.
func Netmask(prefix cty.Value) (cty.Value, error) {
	return NetmaskFunc.Call([]cty.Value{prefix})
}
