package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyze_Copy(t *testing.T) {
	require := require.New(t)

	g := &Generator{
		packageDir:     "../../nomad/structs",
		typeNames:      []string{"Job"},
		methods:        []string{"Job.Copy"},
		excludedFields: []string{"Payload"},
		typeSpecs:      map[string]*TypeSpecNode{},
	}

	pkgs, err := g.loadPackages()
	require.NoError(err)

	err = g.parsePackages(pkgs)
	require.NoError(err)

	g.analyze()

	require.True(g.typeSpecs["Job"].isCopier())
	require.Len(g.typeSpecs["Job"].fields, 3) // 3 pointer fields

	got := func(typespec string) bool {
		return g.typeSpecs[typespec].isCopier()
	}

	require.True(got("Multiregion"), "Multiregion has pointer and array fields")
	require.True(got("PeriodicConfig"), "PeriodicConfig has a pointer field")
	require.True(got("ParameterizedJobConfig"), "ParameterizedJobConfig has array fields")
	require.True(got("UpdateStrategy"), "UpdateStrategy has a Copy method")

	require.False(got("TaskGroupSummary"), "TaskGroupSummary has only primitive fields")
}
