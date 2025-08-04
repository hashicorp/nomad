// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/helper/winsvc"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/mock"
)

func TestWindowsServiceInstallCommand_Run(t *testing.T) {
	t.Parallel()

	freshInstallFn := func(m *winsvc.MockWindowsServiceManager) {
		srv := winsvc.NewMockWindowsService(t)
		m.On("Close").Return(nil).Once()
		m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Times(2)
		m.On("CreateService",
			winsvc.WINDOWS_SERVICE_NAME,
			mock.MatchedBy(func(iface any) bool {
				val := iface.(string)
				return strings.Contains(val, "programfiles") &&
					strings.HasSuffix(val, "nomad.exe")
			}),
			mock.AnythingOfType("winsvc.WindowsServiceConfiguration")).Return(srv, nil).Once()
		srv.On("Close").Return(nil).Once()
		srv.On("EnableEventlog").Return(nil).Once()
		srv.On("Stop").Return(nil).Once()
		srv.On("Start").Return(nil).Once()
	}
	upgradeInstallFn := func(m *winsvc.MockWindowsServiceManager) {
		srv := winsvc.NewMockWindowsService(t)
		m.On("Close").Return(nil).Once()
		m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(true, nil).Times(2)
		m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(srv, nil).Times(2)
		srv.On("Configure",
			mock.AnythingOfType("winsvc.WindowsServiceConfiguration")).Return(nil).Once()
		srv.On("Close").Return(nil).Once()
		srv.On("EnableEventlog").Return(nil).Once()
		srv.On("Stop").Return(nil).Times(2)
		srv.On("Start").Return(nil).Once()
	}

	testCases := []struct {
		desc        string
		args        []string
		privilegeFn func() bool
		setup       func(string, *winsvc.MockWindowsServiceManager)
		after       func(string)
		output      string
		errOutput   string
		status      int
	}{
		{
			desc: "fresh install success",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			output: "Success",
			status: 0,
		},
		{
			desc: "fresh install writes config",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			after: func(dir string) {
				must.FileExists(t, filepath.Join(dir, "programdata/HashiCorp/nomad/config/config.hcl"))
			},
			output: "initial configuration file",
		},
		{
			desc: "fresh install binary file",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			after: func(dir string) {
				must.FileExists(t, filepath.Join(dir, "programfiles/HashiCorp/nomad/bin/nomad.exe"))
			},
			output: "binary installed",
		},
		{
			desc: "fresh install configuration already exists",
			setup: func(dir string, m *winsvc.MockWindowsServiceManager) {
				cdir := filepath.Join(dir, "programdata/HashiCorp/nomad/config")
				err := os.MkdirAll(cdir, 0o755)
				must.NoError(t, err)
				f, err := os.Create(filepath.Join(cdir, "custom.hcl"))
				must.NoError(t, err)
				f.Close()
				freshInstallFn(m)
			},
			after: func(dir string) {
				must.FileNotExists(t, filepath.Join(dir, "programdata/HashiCorp/nomad/config/config.hcl"))
			},
		},
		{
			desc: "fresh install binary already exists",
			setup: func(dir string, m *winsvc.MockWindowsServiceManager) {
				cdir := filepath.Join(dir, "programfiles/HashiCorp/nomad/bin")
				err := os.MkdirAll(cdir, 0o755)
				must.NoError(t, err)
				// create empty binary file
				f, err := os.Create(filepath.Join(cdir, "nomad.exe"))
				must.NoError(t, err)
				f.Close()
				freshInstallFn(m)
			},
			after: func(dir string) {
				s, err := os.Stat(filepath.Join(dir, "programfiles/HashiCorp/nomad/bin/nomad.exe"))
				must.NoError(t, err)
				// ensure binary file is not empty
				must.NonZero(t, s.Size())
			},
		},
		{
			desc: "upgrade install success",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				upgradeInstallFn(m)
			},
			output: "Success",
			status: 0,
		},
		{
			desc:      "with arguments",
			args:      []string{"any", "value"},
			errOutput: "takes no arguments",
			status:    1,
		},
		{
			desc: "with -install-dir",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			args: []string{"-install-dir", "{{.ProgramFiles}}/custom/bin"},
			after: func(dir string) {
				_, err := os.Stat(filepath.Join(dir, "programfiles/custom/bin/nomad.exe"))
				must.NoError(t, err)
			},
		},
		{
			desc: "with -config-dir",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			args: []string{"-config-dir", "{{.ProgramData}}/custom/nomad-configuration"},
			after: func(dir string) {
				_, err := os.Stat(filepath.Join(dir, "programdata/custom/nomad-configuration"))
				must.NoError(t, err)
			},
		},
		{
			desc: "with -data-dir",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				freshInstallFn(m)
			},
			args: []string{"-data-dir", "{{.ProgramData}}/custom/nomad-data"},
			after: func(dir string) {
				_, err := os.Stat(filepath.Join(dir, "programdata/custom/nomad-data"))
				must.NoError(t, err)
			},
		},
		{
			desc: "service registered check failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, errors.New("lookup failure")).Once()
			},
			errOutput: "unable to check for existing service",
			status:    1,
		},
		{
			desc: "service registered check failure during service install",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, errors.New("lookup failure")).Once()
			},
			errOutput: "registration check failed",
			status:    1,
		},
		{
			desc: "get existing service to stop failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(true, nil).Once()
				m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(nil, errors.New("service get failure")).Once()
			},
			errOutput: "could not get existing service",
			status:    1,
		},
		{
			desc: "stop existing service failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(true, nil).Once()
				m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(srv, nil).Once()
				srv.On("Stop").Return(errors.New("cannot stop")).Once()
			},
			errOutput: "unable to stop existing service",
			status:    1,
		},
		{
			desc: "get existing service to configure failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(true, nil).Times(2)
				m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(srv, nil).Once()
				srv.On("Stop").Return(nil)
				m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(nil, errors.New("service get failure")).Once()
			},
			errOutput: "unable to get existing service",
			status:    1,
		},
		{
			desc: "configure service failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				srv.On("Close").Return(nil).Once()
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(true, nil).Times(2)
				m.On("GetService", winsvc.WINDOWS_SERVICE_NAME).Return(srv, nil).Times(2)
				srv.On("Stop").Return(nil).Once()
				srv.On("Configure", mock.Anything).Return(errors.New("configure failure")).Once()
			},
			errOutput: "unable to configure service",
			status:    1,
		},
		{
			desc: "create service failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Times(2)
				m.On("CreateService", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("create service failure")).Once()
			},
			errOutput: "unable to create service",
			status:    1,
		},
		{
			desc: "eventlog setup failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				srv.On("Close").Return(nil).Once()
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Times(2)
				m.On("CreateService", mock.Anything, mock.Anything, mock.Anything).Return(srv, nil).Once()
				srv.On("EnableEventlog").Return(errors.New("eventlog configure failure")).Once()
			},
			errOutput: "could not configure eventlog",
			status:    1,
		},
		{
			desc: "service stop pre-start failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				srv.On("Close").Return(nil).Once()
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Times(2)
				m.On("CreateService", mock.Anything, mock.Anything, mock.Anything).Return(srv, nil).Once()
				srv.On("EnableEventlog").Return(nil).Once()
				srv.On("Stop").Return(errors.New("service stop failure")).Once()
			},
			errOutput: "could not stop service",
			status:    1,
		},
		{
			desc: "service start failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := winsvc.NewMockWindowsService(t)
				srv.On("Close").Return(nil).Once()
				m.On("Close").Return(nil).Once()
				m.On("IsServiceRegistered", winsvc.WINDOWS_SERVICE_NAME).Return(false, nil).Times(2)
				m.On("CreateService", mock.Anything, mock.Anything, mock.Anything).Return(srv, nil).Once()
				srv.On("EnableEventlog").Return(nil).Once()
				srv.On("Stop").Return(nil).Once()
				srv.On("Start").Return(errors.New("service start failure")).Once()
			},
			errOutput: "could not start service",
			status:    1,
		},
		{
			desc:        "not running as administator",
			privilegeFn: func() bool { return false },
			errOutput:   "must be run with Administator privileges",
			status:      1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			testDir, err := os.MkdirTemp(os.TempDir(), "nomad-test")
			must.NoError(t, err)
			defer os.RemoveAll(testDir)

			ui := cli.NewMockUi()
			mgr := winsvc.NewMockWindowsServiceManager(t)
			if tc.setup != nil {
				tc.setup(testDir, mgr)
			}

			pfn := tc.privilegeFn
			if pfn == nil {
				pfn = func() bool { return true }
			}

			cmd := &WindowsServiceInstallCommand{
				Meta: Meta{Ui: ui},
				serviceManagerFn: func() (winsvc.WindowsServiceManager, error) {
					return mgr, nil
				},
				expandPathFn:      createFileExpandFn(testDir),
				privilegedCheckFn: pfn,
			}
			result := cmd.Run(tc.args)

			out := ui.OutputWriter.String()
			outErr := ui.ErrorWriter.String()
			must.Eq(t, result, tc.status)
			if tc.output != "" {
				must.StrContains(t, out, tc.output)
			}
			if tc.errOutput != "" {
				must.StrContains(t, outErr, tc.errOutput)
			}
			if tc.after != nil {
				tc.after(testDir)
			}
		})
	}
}

func createFileExpandFn(rootDir string) func(string) (string, error) {
	return func(v string) (string, error) {
		rpls := struct {
			SystemRoot   string
			SystemDrive  string
			ProgramData  string
			ProgramFiles string
		}{
			SystemRoot:   filepath.Join(rootDir, "systemroot"),
			SystemDrive:  rootDir,
			ProgramData:  filepath.Join(rootDir, "programdata"),
			ProgramFiles: filepath.Join(rootDir, "programfiles"),
		}

		tmpl, err := template.New("expansion").Parse(v)
		if err != nil {
			return "", err
		}
		result := new(bytes.Buffer)
		if err := tmpl.Execute(result, rpls); err != nil {
			return "", err
		}

		return strings.ReplaceAll(result.String(), `\`, "/"), nil
	}
}
