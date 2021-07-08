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

	analyzer, err := NewAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)
	visitor := NewNomadPackageVisitor(analyzer, t.Log, debugOptions)

	parser := PackageParser{
		Visitor: visitor,
	}

	err = parser.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
}
