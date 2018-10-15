package structs

import (
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/shared/structs/proto"
)

func ConvertProtoAttribute(in *proto.Attribute) *Attribute {
	out := &Attribute{
		Unit: in.Unit,
	}

	switch in.Value.(type) {
	case *proto.Attribute_BoolVal:
		out.Bool = helper.BoolToPtr(in.GetBoolVal())
	case *proto.Attribute_FloatVal:
		out.Float = helper.Float64ToPtr(in.GetFloatVal())
	case *proto.Attribute_IntVal:
		out.Int = helper.Int64ToPtr(in.GetIntVal())
	case *proto.Attribute_StringVal:
		out.String = helper.StringToPtr(in.GetStringVal())
	default:
	}

	return out
}

func Pow(a, b int64) int64 {
	var p int64 = 1
	for b > 0 {
		if b&1 != 0 {
			p *= a
		}
		b >>= 1
		a *= a
	}
	return p
}
