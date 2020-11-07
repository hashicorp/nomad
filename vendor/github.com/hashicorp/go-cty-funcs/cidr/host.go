package cidr

import (
	"fmt"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

// HostFunc is a function that calculates a full host IP address within a given
// IP network address prefix.
var HostFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "prefix",
			Type: cty.String,
		},
		{
			Name: "hostnum",
			Type: cty.Number,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		var hostNum int
		if err := gocty.FromCtyValue(args[1], &hostNum); err != nil {
			return cty.UnknownVal(cty.String), err
		}
		_, network, err := net.ParseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), fmt.Errorf("invalid CIDR expression: %s", err)
		}

		ip, err := cidr.Host(network, hostNum)
		if err != nil {
			return cty.UnknownVal(cty.String), err
		}

		return cty.StringVal(ip.String()), nil
	},
})

// Host calculates a full host IP address within a given IP network address prefix.
func Host(prefix, hostnum cty.Value) (cty.Value, error) {
	return HostFunc.Call([]cty.Value{prefix, hostnum})
}
