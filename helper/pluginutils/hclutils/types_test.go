// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclutils_test

import (
	"testing"

	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/stretchr/testify/require"
)

func TestMapStrInt_JsonArrays(t *testing.T) {
	spec := hclspec.NewObject(map[string]*hclspec.Spec{
		"port_map": hclspec.NewAttr("port_map", "list(map(number))", false),
	})

	type PidMapTaskConfig struct {
		PortMap hclutils.MapStrInt `codec:"port_map"`
	}

	parser := hclutils.NewConfigParser(spec)

	expected := PidMapTaskConfig{
		PortMap: map[string]int{
			"http":  80,
			"https": 443,
			"ssh":   25,
		},
	}

	t.Run("hcl case", func(t *testing.T) {
		config := `
config {
  port_map {
    http  = 80
    https = 443
    ssh   = 25
  }
}`
		// Test decoding
		var tc PidMapTaskConfig
		parser.ParseHCL(t, config, &tc)

		require.EqualValues(t, expected, tc)

	})
	jsonCases := []struct {
		name string
		json string
	}{
		{
			"array of map entries",
			`{"Config": {"port_map": [{"http": 80}, {"https": 443}, {"ssh": 25}]}}`,
		},
		{
			"array with one map",
			`{"Config": {"port_map": [{"http": 80, "https": 443, "ssh": 25}]}}`,
		},
		{
			"array of maps",
			`{"Config": {"port_map": [{"http": 80, "https": 443}, {"ssh": 25}]}}`,
		},
	}

	for _, c := range jsonCases {
		t.Run("json:"+c.name, func(t *testing.T) {
			// Test decoding
			var tc PidMapTaskConfig
			parser.ParseJson(t, c.json, &tc)

			require.EqualValues(t, expected, tc)

		})
	}
}

func TestMapStrStr_JsonArrays(t *testing.T) {
	spec := hclspec.NewObject(map[string]*hclspec.Spec{
		"port_map": hclspec.NewAttr("port_map", "list(map(string))", false),
	})

	type PidMapTaskConfig struct {
		PortMap hclutils.MapStrStr `codec:"port_map"`
	}

	parser := hclutils.NewConfigParser(spec)

	expected := PidMapTaskConfig{
		PortMap: map[string]string{
			"http":  "80",
			"https": "443",
			"ssh":   "25",
		},
	}

	t.Run("hcl case", func(t *testing.T) {
		config := `
config {
  port_map {
    http  = "80"
    https = "443"
    ssh   = "25"
  }
}`
		// Test decoding
		var tc PidMapTaskConfig
		parser.ParseHCL(t, config, &tc)

		require.EqualValues(t, expected, tc)

	})
	jsonCases := []struct {
		name string
		json string
	}{
		{
			"array of map entries",
			`{"Config": {"port_map": [{"http": "80"}, {"https": "443"}, {"ssh": "25"}]}}`,
		},
		{
			"array with one map",
			`{"Config": {"port_map": [{"http": "80", "https": "443", "ssh": "25"}]}}`,
		},
		{
			"array of maps",
			`{"Config": {"port_map": [{"http": "80", "https": "443"}, {"ssh": "25"}]}}`,
		},
	}

	for _, c := range jsonCases {
		t.Run("json:"+c.name, func(t *testing.T) {
			// Test decoding
			var tc PidMapTaskConfig
			parser.ParseJson(t, c.json, &tc)

			require.EqualValues(t, expected, tc)

		})
	}
}
