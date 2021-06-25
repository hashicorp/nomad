package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenSchema(t *testing.T) {
	err := genSchema()
	require.NoError(t, err, "TestGenSchema failed")
}

func TestLoadPackages(t *testing.T) {
	err := loadPackages()
	require.NoError(t, err, "TestLoadPackages failed")
}
