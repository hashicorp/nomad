package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPackageVisitorParse(t *testing.T) {
	req := require.New(t)

	debugOptions := defaultDebugOptions
	debugOptions.showSource = false
	debugOptions.showHelpers = true
	debugOptions.showReturnSource = true

	analyzer, err := NewAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)
	visitor := NewNomadPackageVisitor(analyzer, t.Log, debugOptions)

	parser := PackageParser{
		Packages: nomadPackages,
		Visitor:  visitor,
	}

	err = parser.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
}
