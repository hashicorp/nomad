// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"os"
	"testing"

	"github.com/shoenig/test/must"
)

func TestExpandPath(t *testing.T) {
	t.Run("SystemDrive", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			defer resetPathExpansions()
			result, err := ExpandPath(`{{.SystemDrive}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.SystemDrive}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			cval := os.Getenv("SystemDrive")
			defer os.Setenv("SystemDrive", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Setenv("SystemDrive", `z:`))
			result, err := ExpandPath(`{{.SystemDrive}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			cval := os.Getenv("SystemDrive")
			defer os.Setenv("SystemDrive", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Unsetenv("SystemDrive"))
			_, err := ExpandPath(`{{.SystemDrive}}\testing`)
			must.Error(t, err)
		})
	})

	t.Run("ProgramData", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			defer resetPathExpansions()
			result, err := ExpandPath(`{{.ProgramData}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramData}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			cval := os.Getenv("ProgramData")
			defer os.Setenv("ProgramData", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Setenv("ProgramData", `z:`))
			result, err := ExpandPath(`{{.ProgramData}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			cval := os.Getenv("ProgramData")
			defer os.Setenv("ProgramData", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Unsetenv("ProgramData"))
			result, err := ExpandPath(`{{.ProgramData}}\testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramData}}") // should be pulled from registry
		})
	})

	t.Run("ProgramFiles", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			defer resetPathExpansions()
			result, err := ExpandPath(`{{.ProgramFiles}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramFiles}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			cval := os.Getenv("ProgramFiles")
			defer os.Setenv("ProgramFiles", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Setenv("ProgramFiles", `z:`))
			result, err := ExpandPath(`{{.ProgramFiles}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			cval := os.Getenv("ProgramFiles")
			defer os.Setenv("ProgramFiles", cval)
			defer resetPathExpansions()

			must.NoError(t, os.Unsetenv("ProgramFiles"))
			result, err := ExpandPath(`{{.ProgramFiles}}\testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramFiles}}") // should be pulled from registry
		})
	})

	t.Run("missing key", func(t *testing.T) {
		_, err := ExpandPath(`{{.Unknown}}\testing`)
		must.Error(t, err)
	})
}

func resetPathExpansions() {
	pathExpansions = &windowsPathExpansions{}
}
