// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclutils

import (
	"github.com/hashicorp/go-msgpack/codec"
)

// MapStrInt is a wrapper for map[string]int that handles
// deserialization from different hcl2 json representation
// that were supported in Nomad 0.8
type MapStrInt map[string]int

func (s *MapStrInt) CodecEncodeSelf(enc *codec.Encoder) {
	v := []map[string]int{*s}
	enc.MustEncode(v)
}

func (s *MapStrInt) CodecDecodeSelf(dec *codec.Decoder) {
	ms := []map[string]int{}
	dec.MustDecode(&ms)

	r := map[string]int{}
	for _, m := range ms {
		for k, v := range m {
			r[k] = v
		}
	}
	*s = r
}

// MapStrStr is a wrapper for map[string]string that handles
// deserialization from different hcl2 json representation
// that were supported in Nomad 0.8
type MapStrStr map[string]string

func (s *MapStrStr) CodecEncodeSelf(enc *codec.Encoder) {
	v := []map[string]string{*s}
	enc.MustEncode(v)
}

func (s *MapStrStr) CodecDecodeSelf(dec *codec.Decoder) {
	ms := []map[string]string{}
	dec.MustDecode(&ms)

	r := map[string]string{}
	for _, m := range ms {
		for k, v := range m {
			r[k] = v
		}
	}
	*s = r
}
