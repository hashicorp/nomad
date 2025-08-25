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
)

func TestWindowsServiceInstallCommand_Run(t *testing.T) {
	t.Parallel()

	freshInstallFn := func(m *winsvc.MockWindowsServiceManager) {
		srv := m.NewMockWindowsService()
		m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
		m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
		m.ExpectCreateService(winsvc.WINDOWS_SERVICE_NAME, "nomad.exe",
			winsvc.WindowsServiceConfiguration{}, srv, nil)
		srv.ExpectEnableEventlog(nil)
		srv.ExpectStop(nil)
		srv.ExpectStart(nil)
	}
	upgradeInstallFn := func(m *winsvc.MockWindowsServiceManager) {
		srv := m.NewMockWindowsService()
		m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
		m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
		m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
		m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
		srv.ExpectIsRunning(false, nil)
		srv.ExpectConfigure(winsvc.WindowsServiceConfiguration{}, nil)
		srv.ExpectEnableEventlog(nil)
		srv.ExpectStop(nil)
		srv.ExpectStart(nil)
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
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, errors.New("lookup failure"))
			},
			errOutput: "unable to check for existing service",
			status:    1,
		},
		{
			desc: "service registered check failure during service install",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, errors.New("lookup failure"))
			},
			errOutput: "registration check failed",
			status:    1,
		},
		{
			desc: "get existing service to stop failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, nil, errors.New("service get failure"))
			},
			errOutput: "could not get existing service",
			status:    1,
		},
		{
			desc: "stop existing service failure",
			args: []string{"-reinstall"},
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectIsRunning(true, nil)
				srv.ExpectStop(errors.New("cannot stop"))
			},
			errOutput: "unable to stop existing service",
			status:    1,
		},
		{
			desc: "get existing service to configure failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectIsRunning(false, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, nil, errors.New("service get failure"))
			},
			errOutput: "unable to get existing service",
			status:    1,
		},
		{
			desc: "configure service failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				srv.ExpectIsRunning(false, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectConfigure(winsvc.WindowsServiceConfiguration{}, errors.New("configure failure"))
			},
			errOutput: "unable to configure service",
			status:    1,
		},
		{
			desc: "create service failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectCreateService(winsvc.WINDOWS_SERVICE_NAME, "nomad.exe", winsvc.WindowsServiceConfiguration{}, nil, errors.New("create service failure"))
			},
			errOutput: "unable to create service",
			status:    1,
		},
		{
			desc: "eventlog setup failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectCreateService(winsvc.WINDOWS_SERVICE_NAME, "nomad.exe", winsvc.WindowsServiceConfiguration{}, srv, nil)
				srv.ExpectEnableEventlog(errors.New("eventlog configure failure"))
			},
			errOutput: "could not configure eventlog",
			status:    1,
		},
		{
			desc: "service stop pre-start failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectCreateService(winsvc.WINDOWS_SERVICE_NAME, "nomad.exe", winsvc.WindowsServiceConfiguration{}, srv, nil)
				srv.ExpectEnableEventlog(nil)
				srv.ExpectStop(errors.New("service stop failure"))
			},
			errOutput: "could not stop service",
			status:    1,
		},
		{
			desc: "service start failure",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
				m.ExpectCreateService(winsvc.WINDOWS_SERVICE_NAME, "nomad.exe", winsvc.WindowsServiceConfiguration{}, srv, nil)
				srv.ExpectEnableEventlog(nil)
				srv.ExpectStop(nil)
				srv.ExpectStart(errors.New("service start failure"))
			},
			errOutput: "could not start service",
			status:    1,
		},
		{
			desc: "upgrade without -reinstall and service running",
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectIsRunning(true, nil)
			},
			errOutput: "again with -reinstall",
			status:    1,
		},
		{
			desc: "upgrade with -reinstall and service running",
			args: []string{"-reinstall"},
			setup: func(_ string, m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectIsRunning(true, nil)
				srv.ExpectStop(nil)
				srv.ExpectConfigure(winsvc.WindowsServiceConfiguration{}, nil)
				srv.ExpectEnableEventlog(nil)
				srv.ExpectStop(nil)
				srv.ExpectStart(nil)
			},
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
			testDir := t.TempDir()

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
				winPaths:          createWinPaths(testDir),
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

			mgr.AssertExpectations()
		})
	}
}

func createWinPaths(rootDir string) winsvc.WindowsPaths {
	return &testWindowsPaths{
		SystemRoot:   filepath.Join(rootDir, "systemroot"),
		SystemDrive:  rootDir,
		ProgramData:  filepath.Join(rootDir, "programdata"),
		ProgramFiles: filepath.Join(rootDir, "programfiles"),
	}
}

type testWindowsPaths struct {
	SystemRoot   string
	SystemDrive  string
	ProgramData  string
	ProgramFiles string
}

func (t *testWindowsPaths) Expand(path string) (string, error) {
	tmpl, err := template.New("expansion").Option("missingkey=error").Parse(path)
	if err != nil {
		return "", err
	}
	result := new(bytes.Buffer)
	if err := tmpl.Execute(result, t); err != nil {
		return "", err
	}

	return strings.ReplaceAll(result.String(), `\`, "/"), nil
}

func (t *testWindowsPaths) CreateDirectory(path string, _ bool) error {
	return os.MkdirAll(path, 0o755)
}
