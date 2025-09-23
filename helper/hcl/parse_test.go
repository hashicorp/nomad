// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcl

import (
	"reflect"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestParser_Parse_Duration(t *testing.T) {

	// newTestParserWithDuration returns a Parser with the DecodeDuration registered
	// for both value and pointer time.Duration types.
	newParserFn := func() *Parser {
		p := NewParser()
		d := time.Duration(0)
		p.AddExpressionDecoder(reflect.TypeOf(d), DecodeDuration)
		p.AddExpressionDecoder(reflect.TypeOf(&d), DecodeDuration)
		return p
	}

	type testConfig struct {
		Interval time.Duration  `hcl:"interval"`
		Timeout  *time.Duration `hcl:"timeout,optional"`
	}

	t.Run("string durations", func(t *testing.T) {
		src := `
interval = "5s"
timeout  = "2m"
`
		var parsedConfig testConfig
		p := newParserFn()

		diags := p.Parse([]byte(src), &parsedConfig, "durations.hcl")
		must.False(t, diags.HasErrors())
		must.Eq(t, 5*time.Second, parsedConfig.Interval)
		must.Eq(t, 2*time.Minute, *parsedConfig.Timeout)
	})

	t.Run("numeric durations (nanoseconds)", func(t *testing.T) {
		// 5s and 2m expressed directly in nanoseconds
		src := `
interval = 5000000000
timeout  = 120000000000
`
		var parsedConfig testConfig
		p := newParserFn()

		diags := p.Parse([]byte(src), &parsedConfig, "numeric.hcl")
		must.False(t, diags.HasErrors())
		must.Eq(t, 5*time.Second, parsedConfig.Interval)
		must.Eq(t, 2*time.Minute, *parsedConfig.Timeout)
	})

	t.Run("invalid duration string", func(t *testing.T) {
		src := `
	interval = "notaduration"
	`
		var parsedConfig testConfig
		p := newParserFn()

		diags := p.Parse([]byte(src), &parsedConfig, "invalid_string.hcl")
		must.True(t, diags.HasErrors())
		must.Len(t, 1, diags.Errs())
		must.StrContains(t, diags.Error(), "Unsuitable duration value")
	})

	t.Run("wrong type", func(t *testing.T) {
		src := `
	interval = true
	`
		var parsedConfig testConfig
		p := newParserFn()

		diags := p.Parse([]byte(src), &parsedConfig, "wrong_type.hcl")
		must.True(t, diags.HasErrors())
		must.Len(t, 1, diags.Errs())
		must.StrContains(t, diags.Error(), "Unsuitable value: expected a string")
	})
}
