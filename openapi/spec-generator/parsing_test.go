package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParser(t *testing.T) {
	req := require.New(t)

	visitor := &NomadNodeVisitor{
		logger: t.Log,
	}
	parser := PackageParser{
		Packages: nomadPackages,
		Visitor:  visitor,
	}

	err := parser.Parse()
	req.NoError(err, "TestParser.parser.Parse")
}
