package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSSABuild(t *testing.T) {
	req := require.New(t)
	a := &SSAnalyzer{}

	err := a.BuildProgram(SSADebugOptions{writePackages: false}, t.Log)
	req.NoError(err, "TestSSABuild")
}
