package encoding

import (
	"fmt"

	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/proto/go"
)

// ObjectToValue converts a Sentinel object to a protobuf Value structure.
func ObjectToValue(obj object.Object) (*proto.Value, error) {
	switch obj := obj.(type) {
	case *object.BoolObj:
		return &proto.Value{
			Type:  proto.Value_BOOL,
			Value: &proto.Value_ValueBool{ValueBool: obj.Value},
		}, nil

	case *object.IntObj:
		return &proto.Value{
			Type:  proto.Value_INT,
			Value: &proto.Value_ValueInt{ValueInt: obj.Value},
		}, nil

	case *object.StringObj:
		return &proto.Value{
			Type:  proto.Value_STRING,
			Value: &proto.Value_ValueString{ValueString: obj.Value},
		}, nil

	case *object.ListObj:
		vs := make([]*proto.Value, len(obj.Elts))
		for i, elt := range obj.Elts {
			v, err := ObjectToValue(elt)
			if err != nil {
				return nil, err
			}

			vs[i] = v
		}

		return &proto.Value{
			Type: proto.Value_LIST,
			Value: &proto.Value_ValueList{
				ValueList: &proto.Value_List{
					Elems: vs,
				},
			},
		}, nil

	case *object.MapObj:
		vs := make([]*proto.Value_KV, len(obj.Elts))
		for i, elt := range obj.Elts {
			key, err := ObjectToValue(elt.Key)
			if err != nil {
				return nil, err
			}

			v, err := ObjectToValue(elt.Value)
			if err != nil {
				return nil, err
			}

			vs[i] = &proto.Value_KV{
				Key:   key,
				Value: v,
			}
		}

		return &proto.Value{
			Type: proto.Value_MAP,
			Value: &proto.Value_ValueMap{
				ValueMap: &proto.Value_Map{
					Elems: vs,
				},
			},
		}, nil

	default:
		if obj == object.Null {
			return &proto.Value{Type: proto.Value_NULL}, nil
		}

		return nil, fmt.Errorf(
			"unknown object type to convert to value: %s (%T)", obj, obj)
	}
}
