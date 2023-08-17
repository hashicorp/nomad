// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"github.com/hashicorp/nomad/api"
)

// testVariable returns a test variable spec
func testVariable() *api.Variable {
	return &api.Variable{
		Namespace: "default",
		Path:      "test/var",
		Items: map[string]string{
			"keyA": "valueA",
			"keyB": "valueB",
		},
	}
}
