package structs

import (
	"bytes"
	"reflect"

	"github.com/hashicorp/go-msgpack/v2/codec"
)

// TODO: these are just used for plugin_test.go, can we do something else here?

// msgpackHandle is a shared handle for encoding/decoding of structs
var MsgpackHandle = func() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	h.RawToString = true

	// maintain binary format from time prior to upgrading latest ugorji
	h.BasicHandle.TimeNotBuiltin = true

	// Sets the default type for decoding a map into a nil interface{}.
	// This is necessary in particular because we store the driver configs as a
	// nil interface{}.
	h.MapType = reflect.TypeOf(map[string]interface{}(nil))

	// only review struct codec tags
	h.TypeInfos = codec.NewTypeInfos([]string{"codec"})

	return h
}()

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), MsgpackHandle).Decode(out)
}

type MessageType uint8

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(uint8(t))
	err := codec.NewEncoder(&buf, MsgpackHandle).Encode(msg)
	return buf.Bytes(), err
}
