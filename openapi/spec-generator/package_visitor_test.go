package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPackageVisitorParse(t *testing.T) {
	req := require.New(t)

	visitor := &NomadPackageVisitor{
		logger: t.Log,
	}
	parser := PackageParser{
		Packages: nomadPackages,
		Visitor:  visitor,
	}

	err := parser.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
}
