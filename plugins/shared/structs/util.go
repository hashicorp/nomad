package structs

import "github.com/hashicorp/nomad/plugins/shared/structs/proto"

func ConvertProtoAttribute(in *proto.Attribute) *Attribute {
	out := &Attribute{
		Unit: in.Unit,
	}

	switch in.Value.(type) {
	case *proto.Attribute_BoolVal:
		out.Bool = in.GetBoolVal()
	case *proto.Attribute_FloatVal:
		out.Float = in.GetFloatVal()
	case *proto.Attribute_IntVal:
		out.Int = in.GetIntVal()
	case *proto.Attribute_StringVal:
		out.String = in.GetStringVal()
	default:
	}

	return out
}

func Pow(a, b uint64) uint64 {
	var p uint64 = 1
	for b > 0 {
		if b&1 != 0 {
			p *= a
		}
		b >>= 1
		a *= a
	}
	return p
}
