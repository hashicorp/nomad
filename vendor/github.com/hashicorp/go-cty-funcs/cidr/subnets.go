package cidr

import (
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

// SubnetsFunc is similar to SubnetFunc but calculates many consecutive subnet
// addresses at once, rather than just a single subnet extension.
var SubnetsFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{
			Name: "prefix",
			Type: cty.String,
		},
	},
	VarParam: &function.Parameter{
		Name: "newbits",
		Type: cty.Number,
	},
	Type: function.StaticReturnType(cty.List(cty.String)),
	Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
		_, network, err := net.ParseCIDR(args[0].AsString())
		if err != nil {
			return cty.UnknownVal(cty.String), function.NewArgErrorf(0, "invalid CIDR expression: %s", err)
		}
		startPrefixLen, _ := network.Mask.Size()

		prefixLengthArgs := args[1:]
		if len(prefixLengthArgs) == 0 {
			return cty.ListValEmpty(cty.String), nil
		}

		var firstLength int
		if err := gocty.FromCtyValue(prefixLengthArgs[0], &firstLength); err != nil {
			return cty.UnknownVal(cty.String), function.NewArgError(1, err)
		}
		firstLength += startPrefixLen

		retVals := make([]cty.Value, len(prefixLengthArgs))

		current, _ := cidr.PreviousSubnet(network, firstLength)
		for i, lengthArg := range prefixLengthArgs {
			var length int
			if err := gocty.FromCtyValue(lengthArg, &length); err != nil {
				return cty.UnknownVal(cty.String), function.NewArgError(i+1, err)
			}

			if length < 1 {
				return cty.UnknownVal(cty.String), function.NewArgErrorf(i+1, "must extend prefix by at least one bit")
			}
			// For portability with 32-bit systems where the subnet number
			// will be a 32-bit int, we only allow extension of 32 bits in
			// one call even if we're running on a 64-bit machine.
			// (Of course, this is significant only for IPv6.)
			if length > 32 {
				return cty.UnknownVal(cty.String), function.NewArgErrorf(i+1, "may not extend prefix by more than 32 bits")
			}
			length += startPrefixLen
			if length > (len(network.IP) * 8) {
				protocol := "IP"
				switch len(network.IP) * 8 {
				case 32:
					protocol = "IPv4"
				case 128:
					protocol = "IPv6"
				}
				return cty.UnknownVal(cty.String), function.NewArgErrorf(i+1, "would extend prefix to %d bits, which is too long for an %s address", length, protocol)
			}

			next, rollover := cidr.NextSubnet(current, length)
			if rollover || !network.Contains(next.IP) {
				// If we run out of suffix bits in the base CIDR prefix then
				// NextSubnet will start incrementing the prefix bits, which
				// we don't allow because it would then allocate addresses
				// outside of the caller's given prefix.
				return cty.UnknownVal(cty.String), function.NewArgErrorf(i+1, "not enough remaining address space for a subnet with a prefix of %d bits after %s", length, current.String())
			}

			current = next
			retVals[i] = cty.StringVal(current.String())
		}

		return cty.ListVal(retVals), nil
	},
})

// Subnets calculates a sequence of consecutive subnet prefixes that may be of
// different prefix lengths under a common base prefix.
func Subnets(prefix cty.Value, newbits ...cty.Value) (cty.Value, error) {
	args := make([]cty.Value, len(newbits)+1)
	args[0] = prefix
	copy(args[1:], newbits)
	return SubnetsFunc.Call(args)
}
