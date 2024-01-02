// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/shared/structs/proto"
)

func ConvertProtoAttribute(in *proto.Attribute) *Attribute {
	out := &Attribute{
		Unit: in.Unit,
	}

	switch in.Value.(type) {
	case *proto.Attribute_BoolVal:
		out.Bool = pointer.Of(in.GetBoolVal())
	case *proto.Attribute_FloatVal:
		out.Float = pointer.Of(in.GetFloatVal())
	case *proto.Attribute_IntVal:
		out.Int = pointer.Of(in.GetIntVal())
	case *proto.Attribute_StringVal:
		out.String = pointer.Of(in.GetStringVal())
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

// ConvertProtoStatObject converts between a proto and struct StatObject
func ConvertProtoStatObject(in *proto.StatObject) *StatObject {
	if in == nil {
		return nil
	}

	out := &StatObject{
		Nested:     make(map[string]*StatObject, len(in.Nested)),
		Attributes: make(map[string]*StatValue, len(in.Attributes)),
	}

	for k, v := range in.Nested {
		out.Nested[k] = ConvertProtoStatObject(v)
	}

	for k, v := range in.Attributes {
		out.Attributes[k] = ConvertProtoStatValue(v)
	}

	return out
}

// ConvertProtoStatValue converts between a proto and struct StatValue
func ConvertProtoStatValue(in *proto.StatValue) *StatValue {
	if in == nil {
		return nil
	}

	return &StatValue{
		FloatNumeratorVal:   unwrapDouble(in.FloatNumeratorVal),
		FloatDenominatorVal: unwrapDouble(in.FloatDenominatorVal),
		IntNumeratorVal:     unwrapInt64(in.IntNumeratorVal),
		IntDenominatorVal:   unwrapInt64(in.IntDenominatorVal),
		StringVal:           unwrapString(in.StringVal),
		BoolVal:             unwrapBool(in.BoolVal),
		Unit:                in.Unit,
		Desc:                in.Desc,
	}
}

// ConvertStructStatObject converts between a struct and proto StatObject
func ConvertStructStatObject(in *StatObject) *proto.StatObject {
	if in == nil {
		return nil
	}

	out := &proto.StatObject{
		Nested:     make(map[string]*proto.StatObject, len(in.Nested)),
		Attributes: make(map[string]*proto.StatValue, len(in.Attributes)),
	}

	for k, v := range in.Nested {
		out.Nested[k] = ConvertStructStatObject(v)
	}

	for k, v := range in.Attributes {
		out.Attributes[k] = ConvertStructStatValue(v)
	}

	return out
}

// ConvertStructStatValue converts between a struct and proto StatValue
func ConvertStructStatValue(in *StatValue) *proto.StatValue {
	if in == nil {
		return nil
	}

	return &proto.StatValue{
		FloatNumeratorVal:   wrapDouble(in.FloatNumeratorVal),
		FloatDenominatorVal: wrapDouble(in.FloatDenominatorVal),
		IntNumeratorVal:     wrapInt64(in.IntNumeratorVal),
		IntDenominatorVal:   wrapInt64(in.IntDenominatorVal),
		StringVal:           wrapString(in.StringVal),
		BoolVal:             wrapBool(in.BoolVal),
		Unit:                in.Unit,
		Desc:                in.Desc,
	}
}

// Helper functions for proto wrapping

func unwrapDouble(w *wrappers.DoubleValue) *float64 {
	if w == nil {
		return nil
	}

	v := w.Value
	return &v
}

func wrapDouble(v *float64) *wrappers.DoubleValue {
	if v == nil {
		return nil
	}

	return &wrappers.DoubleValue{Value: *v}
}

func unwrapInt64(w *wrappers.Int64Value) *int64 {
	if w == nil {
		return nil
	}

	v := w.Value
	return &v
}

func wrapInt64(v *int64) *wrappers.Int64Value {
	if v == nil {
		return nil
	}

	return &wrappers.Int64Value{Value: *v}
}

func unwrapString(w *wrappers.StringValue) *string {
	if w == nil {
		return nil
	}

	v := w.Value
	return &v
}

func wrapString(v *string) *wrappers.StringValue {
	if v == nil {
		return nil
	}

	return &wrappers.StringValue{Value: *v}
}

func unwrapBool(w *wrappers.BoolValue) *bool {
	if w == nil {
		return nil
	}

	v := w.Value
	return &v
}

func wrapBool(v *bool) *wrappers.BoolValue {
	if v == nil {
		return nil
	}

	return &wrappers.BoolValue{Value: *v}
}
