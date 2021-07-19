package main

import (
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"testing"
)

func TestTypeInspector(t *testing.T) {
	req := require.New(t)

	typeInspector, err := NewTypeInspector(
		&PackageConfig{
			Config: packages.Config{
				Dir:  "../../nomad/structs/",
				Mode: loadMode,
			},
			Pattern:  ".",
			FileName: "structs.go",
		},
		t.Log,
	)

	req.NoError(err)

	err = typeInspector.Inspect("Job")
	req.NoError(err)

	err = typeInspector.Inspect("ServerMember")
	req.NoError(err)

	err = typeInspector.Inspect("JobSummary")
	req.NoError(err)

	typeInspector.printSchemaRefs()
}
