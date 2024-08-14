// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cni

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestConflist_Json(t *testing.T) {
	conf := &Conflist{
		CniVersion: "0.0.1",
		Name:       "test-config",
		Plugins: []any{
			Generic{Type: "test-plugin"},
		},
	}
	bts, err := conf.Json()
	must.NoError(t, err)
	must.Eq(t, `{
	"cniVersion": "0.0.1",
	"name": "test-config",
	"plugins": [
		{
			"type": "test-plugin"
		}
	]
}`, string(bts))
}
