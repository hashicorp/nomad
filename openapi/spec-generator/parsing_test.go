package main

import (
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"testing"
)

func TestParser(t *testing.T) {
	req := require.New(t)

	visitor := &NomadNodeVisitor{
		logger: t.Log,
	}
	parser := PackageParser{
		Config: packages.Config{
			Dir:  "../../command/agent/",
			Mode: loadMode,
		},
		Pattern: ".",
		Visitor: visitor,
	}

	_, err := parser.Parse()
	req.NoError(err, "TestParser.parser.Parse")
}
