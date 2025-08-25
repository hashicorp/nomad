// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func NewWindowsPaths() WindowsPaths {
	return &windowsPaths{}
}

type windowsPaths struct {
	SystemRoot   string
	SystemDrive  string
	ProgramData  string
	ProgramFiles string
	loadErr      error
	o            sync.Once
}

func (w *windowsPaths) Expand(path string) (string, error) {
	if err := w.load(); err != nil {
		return "", err
	}

	tmpl := template.New("expansion").Option("missingkey=error")
	tmpl, err := tmpl.Parse(path)
	if err != nil {
		return "", err
	}
	result := new(bytes.Buffer)
	if err := tmpl.Execute(result, w); err != nil {
		return "", err
	}

	return result.String(), nil
}

func (w *windowsPaths) CreateDirectory(path string, restrict_on_create bool) error {
	s, err := os.Stat(path)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err == nil {
		// Directory exists so nothing to do
		if s.IsDir() {
			return nil
		}

		return fmt.Errorf("path exists and is not directory - %s", path)
	}

	// NOTE: mode ignored on Windows. If directory should
	// be restricted, an ACL will be applied below.
	if err := os.MkdirAll(path, 0o000); err != nil {
		return err
	}

	// Since the directory was just created, apply access
	// restrictions if requested
	if restrict_on_create {
		if err := setDirectoryPermissions(path); err != nil {
			return err
		}
	}

	return nil
}

func getUserGroupSIDs() (usid *windows.SID, gsid *windows.SID, err error) {
	// NOTE: this token is a pseudo-token and does not
	// need to be closed
	token := windows.GetCurrentProcessToken()

	userToken, err := token.GetTokenUser()
	if err != nil {
		return
	}
	usid = userToken.User.Sid

	userGroup, err := token.GetTokenPrimaryGroup()
	if err != nil {
		return
	}
	gsid = userGroup.PrimaryGroup

	return
}

func setDirectoryPermissions(path string) error {
	// Grab the user and group SID for who is running the process
	userSid, groupSid, err := getUserGroupSIDs()
	if err != nil {
		return err
	}

	// Generate a SID for the administators group
	gsid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}

	// Create an ACL with an ACE for user SID and an ACE for the
	// administrators group SID, both of which are granted full
	// access. No other ACEs are provided which restricts access
	// from non-administrators
	dacl, err := windows.ACLFromEntries(
		[]windows.EXPLICIT_ACCESS{
			{
				AccessPermissions: windows.GENERIC_ALL,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
				Trustee: windows.TRUSTEE{
					MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
					TrusteeForm:              windows.TRUSTEE_IS_SID,
					TrusteeType:              windows.TRUSTEE_IS_USER,
					TrusteeValue:             windows.TrusteeValueFromSID(userSid),
				},
			},
			{
				AccessPermissions: windows.GENERIC_ALL,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
				Trustee: windows.TRUSTEE{
					MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
					TrusteeForm:              windows.TRUSTEE_IS_SID,
					TrusteeType:              windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
					TrusteeValue:             windows.TrusteeValueFromSID(gsid),
				},
			},
		}, nil,
	)
	if err != nil {
		return err
	}

	// Apply the ACL to the directory
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|
			windows.GROUP_SECURITY_INFORMATION|
			windows.DACL_SECURITY_INFORMATION|
			windows.PROTECTED_DACL_SECURITY_INFORMATION,
		userSid, groupSid, dacl, nil); err != nil {
		return err
	}

	return nil
}

func (w *windowsPaths) load() error {
	w.o.Do(func() {
		w.SystemDrive = os.Getenv("SystemDrive")
		if w.SystemDrive == "" {
			w.loadErr = fmt.Errorf("cannot detect Windows SystemDrive path")
			return
		}
		w.SystemRoot = strings.ReplaceAll(os.Getenv("SystemDrive"), "SystemDrive", w.SystemDrive)

		w.ProgramData = os.Getenv("ProgramData")
		if w.ProgramData == "" {
			pdKey, err := registry.OpenKey(registry.LOCAL_MACHINE,
				`SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`, registry.QUERY_VALUE)
			if err == nil {
				if pdVal, _, err := pdKey.GetStringValue("ProgramData"); err == nil {
					w.ProgramData = pdVal
				}
			}
		}
		if w.ProgramData == "" {
			w.loadErr = fmt.Errorf("cannot detect Windows ProgramData path")
			return
		}
		w.ProgramData = strings.ReplaceAll(w.ProgramData, "SystemDrive", w.SystemDrive)

		w.ProgramFiles = os.Getenv("ProgramFiles")
		if w.ProgramFiles == "" {
			pdKey, err := registry.OpenKey(registry.LOCAL_MACHINE,
				`SOFTWARE\Microsoft\Windows\CurrentVersion`, registry.QUERY_VALUE)
			if err == nil {
				if pdVal, _, err := pdKey.GetStringValue("ProgramFilesDir"); err == nil {
					w.ProgramFiles = pdVal
				}
			}
		}
		if w.ProgramFiles == "" {
			w.loadErr = fmt.Errorf("cannot detect Windows ProgramFiles path")
			return
		}
		w.ProgramFiles = strings.ReplaceAll(w.ProgramFiles, "SystemDrive", w.SystemDrive)
	})

	return w.loadErr
}
