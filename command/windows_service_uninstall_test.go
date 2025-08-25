// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/helper/winsvc"
	"github.com/shoenig/test/must"
)

func TestWindowsServiceUninstallCommand_Run(t *testing.T) {
	testCases := []struct {
		desc        string
		args        []string
		privilegeFn func() bool
		setup       func(*winsvc.MockWindowsServiceManager)
		output      string
		errOutput   string
		status      int
	}{
		{
			desc: "service installed",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectStop(nil)
				srv.ExpectDisableEventlog(nil)
				srv.ExpectDelete(nil)
			},
			output: "uninstalled nomad",
		},
		{
			desc: "service not installed",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, nil)
			},
			output: "uninstalled nomad",
		},
		{
			desc: "service registration check failure",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, false, errors.New("registered check failure"))
			},
			errOutput: "unable to check for existing service",
			status:    1,
		},
		{
			desc: "get service failure",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, nil, errors.New("get service failure"))
			},
			errOutput: "could not get existing service",
			status:    1,
		},
		{
			desc: "service stop failure",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectStop(errors.New("service stop failure"))
			},
			errOutput: "unable to stop service",
			status:    1,
		},
		{
			desc: "disable eventlog failure",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectStop(nil)
				srv.ExpectDisableEventlog(errors.New("disable eventlog failure"))
			},
			errOutput: "could not remove eventlog configuration",
			status:    1,
		},
		{
			desc: "delete service failure",
			setup: func(m *winsvc.MockWindowsServiceManager) {
				srv := m.NewMockWindowsService()
				m.ExpectIsServiceRegistered(winsvc.WINDOWS_SERVICE_NAME, true, nil)
				m.ExpectGetService(winsvc.WINDOWS_SERVICE_NAME, srv, nil)
				srv.ExpectStop(nil)
				srv.ExpectDisableEventlog(nil)
				srv.ExpectDelete(errors.New("service delete failure"))
			},
			errOutput: "could not delete service",
			status:    1,
		},
		{
			desc:      "with arguments",
			args:      []string{"any", "value"},
			errOutput: "command takes no arguments",
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
			ui := cli.NewMockUi()
			mgr := winsvc.NewMockWindowsServiceManager(t)
			if tc.setup != nil {
				tc.setup(mgr)
			}

			pfn := tc.privilegeFn
			if pfn == nil {
				pfn = func() bool { return true }
			}

			cmd := &WindowsServiceUninstallCommand{
				Meta: Meta{Ui: ui},
				serviceManagerFn: func() (winsvc.WindowsServiceManager, error) {
					return mgr, nil
				},
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

			mgr.AssertExpectations()
		})
	}
}
