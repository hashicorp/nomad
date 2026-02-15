package main

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestAnalyze_Copy(t *testing.T) {

	logger := testlog.HCLogger(t)

	pkgs, err := loadPackages(logger, "./testdata")
	must.NoError(t, err)

	g := NewGenerator("./testdata", logger)
	g.files = pkgs[0].Files
	g.fileset = pkgs[0].FileSet

	results, err := analyze(logger, pkgs[0])
	must.NoError(t, err)

	var copyResult *Result
	for _, result := range results {
		if result.Method == "Copy" {
			copyResult = result
			break
		}
	}
	must.NotNil(t, copyResult)

	types := map[string]*TypeSpecNode{}
	for _, typeSpecNode := range copyResult.Targets {
		types[typeSpecNode.Name] = typeSpecNode
	}

	must.MapContainsKey(t, types, "Job")
	must.MapContainsKey(t, types, "Multiregion")
	must.MapContainsKey(t, types, "MultiregionStrategy")
	must.MapContainsKey(t, types, "MultiregionRegion")
	must.MapContainsKey(t, types, "PeriodicConfig")
	must.MapContainsKey(t, types, "ParameterizedJobConfig")
	must.MapContainsKey(t, types, "TaskGroupSummary",
		must.Sprint("TaskGroupSummary should have a Copy method even though it has only primitive fields"))

	must.MapContainsKey(t, types, "Service",
		must.Sprint("Service is not a child of Job but is Copy"))

	must.MapNotContainsKey(t, types, "UpdateStrategy",
		must.Sprint("UpdateStrategy has a Copy method"))

	must.True(t, types["Job"].IsCopy())
	must.Len(t, 9, types["Job"].Fields)
}
