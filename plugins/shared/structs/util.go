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

func ConvertProtoAttributeMap(in map[string]*proto.Attribute) map[string]*Attribute {
	if in == nil {
		return nil
	}

	out := make(map[string]*Attribute, len(in))
	for k, a := range in {
		out[k] = ConvertProtoAttribute(a)
	}

	return out
}

func ConvertStructsAttribute(in *Attribute) *proto.Attribute {
	out := &proto.Attribute{
		Unit: in.Unit,
	}

	if in.Int != nil {
		out.Value = &proto.Attribute_IntVal{
			IntVal: *in.Int,
		}
	} else if in.Float != nil {
		out.Value = &proto.Attribute_FloatVal{
			FloatVal: *in.Float,
		}
	} else if in.String != nil {
		out.Value = &proto.Attribute_StringVal{
			StringVal: *in.String,
		}
	} else if in.Bool != nil {
		out.Value = &proto.Attribute_BoolVal{
			BoolVal: *in.Bool,
		}
	}

	return out
}

func ConvertStructAttributeMap(in map[string]*Attribute) map[string]*proto.Attribute {
	if in == nil {
		return nil
	}

	out := make(map[string]*proto.Attribute, len(in))
	for k, a := range in {
		out[k] = ConvertStructsAttribute(a)
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

// CopyMapStringAttribute copies a map of string to Attribute
func CopyMapStringAttribute(in map[string]*Attribute) map[string]*Attribute {
	if in == nil {
		return nil
	}

	out := make(map[string]*Attribute, len(in))
	for k, v := range in {
		out[k] = v.Copy()
	}
	return out
}
