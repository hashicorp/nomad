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

	visitor := &NomadPackageVisitor{
		logger:       t.Log,
		debugOptions: debugOptions,
	}

	parser := PackageParser{
		Packages: nomadPackages,
		Visitor:  visitor,
	}

	err := parser.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
}
