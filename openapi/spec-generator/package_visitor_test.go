package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPackageVisitorParse(t *testing.T) {
	req := require.New(t)

	debugOptions := defaultDebugOptions
	debugOptions.printSource = false
	debugOptions.printHelpers = true
	debugOptions.printReturnSource = true
	debugOptions.filterByMethods = []string{"agent.jobListRequest"}

	analyzer, err := newAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)
	visitor := newNomadPackageVisitor(analyzer, t.Log, debugOptions)

	err = visitor.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
}
