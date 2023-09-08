// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestDataFormat(t *testing.T) {
	ci.Parallel(t)
	type testData struct {
		Region string
		ID     string
		Name   string
	}

	var tData = testData{"global", "1", "example"}

	// Note: this variable is space indented (4) and requires the final brace to
	// be at char 1
	const expectJSON = `{
    "ID": "1",
    "Name": "example",
    "Region": "global"
}`

	var tcs = map[string]struct {
		format   string
		template string
		expect   string
		isError  bool
	}{
		"json_good": {
			format:   "json",
			template: "",
			expect:   expectJSON,
		},
		"template_good": {
			format:   "template",
			template: "{{.Region}}",
			expect:   "global",
		},
		"template_bad": {
			format:   "template",
			template: "{{.foo}}",
			isError:  true,
			expect:   "can't evaluate field foo",
		},
		"template_empty": {
			format:   "template",
			template: "",
			isError:  true,
			expect:   "template needs to be specified in golang's text/template format.",
		},
		"template_sprig": {
			format:   "template",
			template: `{{$a := 1}}{{ $a | sprig_add 1 }}`,
			expect:   "2",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tc := tc
			ci.Parallel(t)
			fm, err := DataFormat(tc.format, tc.template)
			must.NoError(t, err)
			result, err := fm.TransformData(tData)
			if tc.isError {
				must.ErrorContains(t, err, tc.expect)
				return
			}
			must.NoError(t, err)
			must.Eq(t, tc.expect, result)
		})
	}
}
