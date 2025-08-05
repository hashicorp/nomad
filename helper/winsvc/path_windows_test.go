// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/windows"
)

func TestCreateDirectory(t *testing.T) {
	ci.Parallel(t)
	testDir := t.TempDir()

	t.Run("create", func(t *testing.T) {
		// NOTE: parallel is not set here to force parent
		// to wait for subtests to complete
		t.Run("unrestricted", func(t *testing.T) {
			ci.Parallel(t)
			path := filepath.Join(testDir, t.Name())

			err := NewWindowsPaths().CreateDirectory(path, false)
			must.NoError(t, err)

			dacl := getDirectoryDACL(t, path)

			// When not applying restrictions on the new directory, all
			// ACEs will be inherited from the parent
			for i := range dacl.AceCount {
				ace := &windows.ACCESS_ALLOWED_ACE{}
				must.NoError(t, windows.GetAce(dacl, uint32(i), &ace), must.Sprint("failed to load ACE"))
				must.Eq(t, windows.INHERITED_ACCESS_ENTRY, ace.Header.AceFlags&windows.INHERITED_ACCESS_ENTRY,
					must.Sprint("ACE is not inherited"))
			}
		})

		t.Run("restricted", func(t *testing.T) {
			ci.Parallel(t)
			path := filepath.Join(testDir, t.Name())

			err := NewWindowsPaths().CreateDirectory(path, true)
			must.NoError(t, err)

			dacl := getDirectoryDACL(t, path)
			matches := map[string]struct{}{}

			// When restrictions are applied on the new directory, all
			// ACEs will be directly applied.
			for i := range dacl.AceCount {
				ace := &windows.ACCESS_ALLOWED_ACE{}
				must.NoError(t, windows.GetAce(dacl, uint32(i), &ace), must.Sprint("failed to load ACE"))
				must.NotEq(t, windows.INHERITED_ACCESS_ENTRY, ace.Header.AceFlags&windows.INHERITED_ACCESS_ENTRY,
					must.Sprint("ACE should not be inherited"))

				if ace.Mask&windows.GENERIC_ALL == windows.GENERIC_ALL {
					sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
					matches[sid.String()] = struct{}{}
				}
			}

			// All privileges should be set for user and administrators groups
			adminGroupSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
			must.NoError(t, err, must.Sprint("failed to create well known administrators group SID"))
			userSID, _, err := getUserGroupSIDs()
			must.NoError(t, err, must.Sprint("failed to get user SID"))

			must.NotNil(t, matches[userSID.String()], must.Sprint("missing user ACE with GENERIC_ALL"))
			must.NotNil(t, matches[adminGroupSID.String()],
				must.Sprint("missing administrators group ACE with GENERIC_ALL"))

			must.Eq(t, 2, len(matches), must.Sprint("unexpected GENERIC_ALL ACEs found"))
		})

		t.Run("unrestricted already exists", func(t *testing.T) {
			ci.Parallel(t)
			path := filepath.Join(testDir, t.Name())
			must.NoError(t, os.MkdirAll(path, 0o000))

			err := NewWindowsPaths().CreateDirectory(path, false)
			must.NoError(t, err)

			dacl := getDirectoryDACL(t, path)

			// No restrictions are applied, so check that all ACEs
			// are inherited from parent
			for i := range dacl.AceCount {
				ace := &windows.ACCESS_ALLOWED_ACE{}
				must.NoError(t, windows.GetAce(dacl, uint32(i), &ace), must.Sprint("failed to load ACE"))
				must.Eq(t, windows.INHERITED_ACCESS_ENTRY, ace.Header.AceFlags&windows.INHERITED_ACCESS_ENTRY,
					must.Sprint("ACE is not inherited"))
			}
		})

		t.Run("restricted already exists", func(t *testing.T) {
			ci.Parallel(t)
			path := filepath.Join(testDir, t.Name())
			must.NoError(t, os.MkdirAll(path, 0o000))

			err := NewWindowsPaths().CreateDirectory(path, true)
			must.NoError(t, err)

			dacl := getDirectoryDACL(t, path)

			// When the directory already exists, restrictions should not
			// be applied so validate that all ACEs are inherited
			for i := range dacl.AceCount {
				ace := &windows.ACCESS_ALLOWED_ACE{}
				must.NoError(t, windows.GetAce(dacl, uint32(i), &ace), must.Sprint("failed to load ACE"))
				must.Eq(t, windows.INHERITED_ACCESS_ENTRY, ace.Header.AceFlags&windows.INHERITED_ACCESS_ENTRY,
					must.Sprint("ACE is not inherited"))
			}
		})
	})
}

func TestExpand(t *testing.T) {
	t.Run("SystemDrive", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			result, err := NewWindowsPaths().Expand(`{{.SystemDrive}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.SystemDrive}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			t.Setenv("SystemDrive", `z:`)
			result, err := NewWindowsPaths().Expand(`{{.SystemDrive}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			t.Setenv("SystemDrive", "")
			_, err := NewWindowsPaths().Expand(`{{.SystemDrive}}\testing`)
			must.ErrorContains(t, err, "cannot detect Windows SystemDrive path")
		})
	})

	t.Run("ProgramData", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			result, err := NewWindowsPaths().Expand(`{{.ProgramData}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramData}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			t.Setenv("ProgramData", `z:`)
			result, err := NewWindowsPaths().Expand(`{{.ProgramData}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			t.Setenv("ProgramData", "")
			result, err := NewWindowsPaths().Expand(`{{.ProgramData}}\testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramData}}") // should be pulled from registry
		})
	})

	t.Run("ProgramFiles", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			result, err := NewWindowsPaths().Expand(`{{.ProgramFiles}}/testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramFiles}}")
		})
		t.Run("custom environment variable", func(t *testing.T) {
			t.Setenv("ProgramFiles", `z:`)
			result, err := NewWindowsPaths().Expand(`{{.ProgramFiles}}\testing`)
			must.NoError(t, err)
			must.Eq(t, `z:\testing`, result)
		})
		t.Run("unset environment variable", func(t *testing.T) {
			t.Setenv("ProgramFiles", "")
			result, err := NewWindowsPaths().Expand(`{{.ProgramFiles}}\testing`)
			must.NoError(t, err)
			must.StrNotContains(t, result, "{{.ProgramFiles}}") // should be pulled from registry
		})
	})

	t.Run("missing key", func(t *testing.T) {
		_, err := NewWindowsPaths().Expand(`{{.Unknown}}\testing`)
		must.ErrorContains(t, err, "can't evaluate field")
	})
}

func getDirectoryDACL(t *testing.T, path string) *windows.ACL {
	t.Helper()

	s, err := os.Stat(path)
	must.NoError(t, err)
	must.True(t, s.IsDir(), must.Sprint("expected path to be a directory"))

	info, err := windows.GetNamedSecurityInfo(path,
		windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	must.NoError(t, err, must.Sprint("failed to get path security information"))

	dacl, _, err := info.DACL()
	must.NoError(t, err, must.Sprint("failed to get path ACL"))

	return dacl
}
