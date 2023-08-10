// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestMaybeDecodeTimeIgnoresASCII(t *testing.T) {
	cases := []string{
		"127.0.0.1/32",
		"host",
	}

	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			tt, err := maybeDecodeTime(c)
			fmt.Println(tt)
			require.Nil(t, tt)
			require.Error(t, err)
		})
	}
}

func TestDecodesTime(t *testing.T) {
	ci.Parallel(t)

	type Value struct {
		CreateTime time.Time
		Mode       string
	}
	now := time.Now().Truncate(time.Second)
	v := Value{
		CreateTime: now,
		Mode:       "host",
	}

	var buf bytes.Buffer
	err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(v)
	require.NoError(t, err)

	var r map[string]interface{}
	err = codec.NewDecoder(&buf, structs.MsgpackHandle).Decode(&r)
	require.NoError(t, err)

	require.Equal(t, "host", r["Mode"])
	require.IsType(t, "", r["CreateTime"])

	fixTime(r)

	expected := map[string]interface{}{
		"CreateTime": now,
		"Mode":       "host",
	}
	require.Equal(t, expected, r)
}

func TestMyDate(t *testing.T) {
	ci.Parallel(t)

	handler := &codec.MsgpackHandle{}
	handler.TimeNotBuiltin = true

	d := time.Date(2025, 7, 10, 8, 1, 56, 0, time.UTC)

	var buf bytes.Buffer
	err := codec.NewEncoder(&buf, handler).Encode(d)
	require.NoError(t, err)

	var s string
	err = codec.NewDecoder(&buf, handler).Decode(&s)
	require.NoError(t, err)

	fmt.Printf("Original:    %q\nround trips: %q\n", d, s)
}
