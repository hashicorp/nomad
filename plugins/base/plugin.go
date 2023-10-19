// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"bytes"
	"context"
	"reflect"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base/proto"
	"google.golang.org/grpc"
)

const (
	// PluginTypeBase implements the base plugin interface
	PluginTypeBase = "base"

	// PluginTypeDriver implements the driver plugin interface
	PluginTypeDriver = "driver"

	// PluginTypeDevice implements the device plugin interface
	PluginTypeDevice = "device"
)

var (
	// Handshake is a common handshake that is shared by all plugins and Nomad.
	Handshake = plugin.HandshakeConfig{
		// ProtocolVersion for the executor protocol.
		// Version 1: pre 0.9 netrpc based executor
		// Version 2: 0.9+ grpc based executor
		ProtocolVersion:  2,
		MagicCookieKey:   "NOMAD_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "e4327c2e01eabfd75a8a67adb114fb34a757d57eee7728d857a8cec6e91a7255",
	}
)

// PluginBase is wraps a BasePlugin and implements go-plugins GRPCPlugin
// interface to expose the interface over gRPC.
type PluginBase struct {
	plugin.NetRPCUnsupportedPlugin
	Impl BasePlugin
}

func (p *PluginBase) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterBasePluginServer(s, &basePluginServer{
		impl:   p.Impl,
		broker: broker,
	})
	return nil
}

func (p *PluginBase) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &BasePluginClient{
		Client:  proto.NewBasePluginClient(c),
		DoneCtx: ctx,
	}, nil
}

// MsgpackHandle is a shared handle for encoding/decoding of structs
var MsgpackHandle = func() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	h.RawToString = true

	// maintain binary format from time prior to upgrading latest ugorji
	h.BasicHandle.TimeNotBuiltin = true

	h.MapType = reflect.TypeOf(map[string]interface{}(nil))

	// only review struct codec tags - ignore `json` flags
	h.TypeInfos = codec.NewTypeInfos([]string{"codec"})

	return h
}()

// MsgPackDecode is used to decode a MsgPack encoded object
func MsgPackDecode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), MsgpackHandle).Decode(out)
}

// MsgPackEncode is used to encode an object to MsgPack
func MsgPackEncode(b *[]byte, in interface{}) error {
	return codec.NewEncoderBytes(b, MsgpackHandle).Encode(in)
}
