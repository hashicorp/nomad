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
