// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"io/fs"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestWindowsServiceManager(t *testing.T) {
	t.Parallel()

	t.Run("IsServiceRegistered", func(t *testing.T) {
		t.Parallel()
		t.Run("service does not exist", func(t *testing.T) {
			t.Parallel()
			_, manager, closer := makeManagers()
			defer closer()

			result, err := manager.IsServiceRegistered("fake-service-name")
			must.NoError(t, err, must.Sprint("check should not error"))
			must.False(t, result, must.Sprint("service should not exist"))
		})

		t.Run("service does exist", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()

			result, err := manager.IsServiceRegistered(serviceName)
			must.NoError(t, err, must.Sprint("check should not error"))
			must.True(t, result, must.Sprint("service should exist"))
		})
	})

	t.Run("GetService", func(t *testing.T) {
		t.Parallel()
		t.Run("service does not exist", func(t *testing.T) {
			t.Parallel()
			_, manager, closer := makeManagers()
			defer closer()
			_, err := manager.GetService("fake-service-name")
			must.Error(t, err, must.Sprint("error should be generated when service does not exist"))
		})

		t.Run("service does exist", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()

			srv, err := manager.GetService(serviceName)
			must.NoError(t, err)
			defer srv.Close()
			must.Eq(t, serviceName, srv.Name(), must.Sprint("service name does not match"))
		})
	})

	t.Run("CreateService", func(t *testing.T) {
		t.Parallel()
		t.Run("service does not exist", func(t *testing.T) {
			t.Parallel()
			serviceName := generateServiceName()
			m, manager, closer := makeManagers()
			defer closer()

			srv, err := manager.CreateService(serviceName, `c:\stub`, WindowsServiceConfiguration{})
			must.NoError(t, err)
			defer srv.Close()
			defer deleteStubService(m, serviceName)

			must.Eq(t, serviceName, srv.Name(), must.Sprint("new service name is incorrect"))
		})

		t.Run("service does exist", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()

			_, err := manager.CreateService(serviceName, `c:\stub`, WindowsServiceConfiguration{})
			must.Error(t, err, must.Sprint("service creation should fail"))
		})

		t.Run("with configuration", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName := generateServiceName()
			srv, err := manager.CreateService(serviceName, `c:\stub`,
				WindowsServiceConfiguration{DisplayName: "testing service", StartType: StartDisabled})
			must.NoError(t, err, must.Sprint("service should be created"))
			defer srv.Close()
			defer deleteStubService(m, serviceName)

			directSrv, err := m.OpenService(serviceName)
			must.NoError(t, err, must.Sprint("direct service connection should succeed"))
			defer directSrv.Close()

			config, err := directSrv.Config()
			must.NoError(t, err, must.Sprint("configuration should be available from service"))
			must.Eq(t, "testing service", config.DisplayName, must.Sprint("new service name does not match"))
		})
	})
}

// This is a simple service available in Windows. It will
// be used to locate the executable so a test service can
// be created using it that will allow proper start/stop
// testing.
const TEST_WINDOWS_SERVICE = "SNMPTrap"

func TestWindowsService(t *testing.T) {
	t.Parallel()

	mg, _, closer := makeManagers()
	defer closer()
	snmpSvc, err := mg.OpenService(TEST_WINDOWS_SERVICE)
	must.NoError(t, err)
	defer snmpSvc.Close()
	snmpConfig, err := snmpSvc.Config()
	must.NoError(t, err)
	binPath := snmpConfig.BinaryPathName

	runnableSvcFn := func() (WindowsService, func()) {
		_, manager, closer := makeManagers()
		defer closer()
		runnableSvc, err := manager.CreateService(generateServiceName(), binPath,
			WindowsServiceConfiguration{StartType: StartManual, BinaryPathName: binPath})
		must.NoError(t, err, must.Sprint("failed to create runnable service"))
		return runnableSvc, func() { runnableSvc.Close() }
	}

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		m, manager, closer := makeManagers()
		defer closer()

		serviceName, closer := generateStubService(m)
		defer closer()
		srv, err := manager.GetService(serviceName)
		must.NoError(t, err)
		defer srv.Close()

		must.Eq(t, serviceName, srv.Name(), must.Sprint("service name does not match"))
	})

	t.Run("Configure", func(t *testing.T) {
		t.Parallel()
		t.Run("valid configuration", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()

			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			err = srv.Configure(WindowsServiceConfiguration{
				StartType:      StartDisabled,
				DisplayName:    "testing display name",
				BinaryPathName: `c:\stub -with -arguments`,
			})
			must.NoError(t, err, must.Sprint("valid configuration should not error"))
			directSrv, err := m.OpenService(serviceName)
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()
			config, err := directSrv.Config()
			must.NoError(t, err, must.Sprint("direct service config should be available"))
			must.Eq(t, "testing display name", config.DisplayName, must.Sprint("display name does not match"))
			must.Eq(t, `c:\stub -with -arguments`, config.BinaryPathName, must.Sprint("binary path name does not match"))
		})

		t.Run("invalid configuration", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()

			serviceName, closer := generateStubService(m)
			defer closer()
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			err = srv.Configure(WindowsServiceConfiguration{
				DisplayName:    "testing display name",
				BinaryPathName: `c:\stub -with -arguments`,
			})
			must.Error(t, err, must.Sprint("invalid configuration should error"))
		})
	})

	t.Run("Start", func(t *testing.T) {
		t.Parallel()
		t.Run("when stopped", func(t *testing.T) {
			t.Parallel()
			m, _, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped
				})
				must.True(t, result, must.Sprint("service must be stopped"))
			}
			must.NoError(t, runnableSvc.Start(), must.Sprint("service should start without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Running, must.Sprint("service should be running"))
		})

		t.Run("when running", func(t *testing.T) {
			t.Parallel()
			m, _, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running
				})
				must.True(t, result, must.Sprint("service must be running"))
			}
			must.NoError(t, runnableSvc.Start(), must.Sprint("service should start without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Running, must.Sprint("service should be running"))
		})
	})

	t.Run("Stop", func(t *testing.T) {
		t.Parallel()
		t.Run("when stopped", func(t *testing.T) {
			t.Parallel()
			m, _, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped
				})
				must.True(t, result, must.Sprint("service must be stopped"))
			}
			must.NoError(t, runnableSvc.Stop(), must.Sprint("service should stop without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Stopped, must.Sprint("service should be stopped"))
		})

		t.Run("when running", func(t *testing.T) {
			t.Parallel()
			m, _, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running
				})
				must.True(t, result, must.Sprint("service must be running"))
			}
			must.NoError(t, runnableSvc.Stop(), must.Sprint("service should stop without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Stopped, must.Sprint("service should be stopped"))
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		t.Run("when service exists", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()

			serviceName, _ := generateStubService(m)
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be avaialble"))
			defer srv.Close()

			must.NoError(t, srv.Delete(), must.Sprint("service should be deleted"))
		})

		t.Run("when service does not exist", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()

			serviceName, _ := generateStubService(m)
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be avaialble"))
			defer srv.Close()
			// Delete the service directly
			directSrv, err := m.OpenService(serviceName)
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()
			must.NoError(t, directSrv.Delete(), must.Sprint("service should be deleted"))

			must.Error(t, srv.Delete(), must.Sprint("service should have already been deleted"))
		})
	})

	t.Run("IsRunning", func(t *testing.T) {
		t.Parallel()
		t.Run("when service is not running", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped
				})
				must.True(t, result, must.Sprint("service must be stopped"))
			}

			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsRunning()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.False(t, result, must.Sprint("should not show service as running"))
		})

		t.Run("when service is running", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running
				})
				must.True(t, result, must.Sprint("service must be running"))
			}
			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsRunning()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.True(t, result, must.Sprint("should show service as running"))
		})
	})

	t.Run("IsStopped", func(t *testing.T) {
		t.Parallel()
		t.Run("when service is not running", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped
				})
				must.True(t, result, must.Sprint("service must be stopped"))
			}

			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsStopped()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.True(t, result, must.Sprint("should show service as stopped"))
		})

		t.Run("when service is running", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			runnableSvc, closer := runnableSvcFn()
			defer closer()
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				result := waitForCondition(func() bool {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running
				})
				must.True(t, result, must.Sprint("service must be running"))
			}
			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsStopped()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.False(t, result, must.Sprint("should not show service as stopped"))
		})
	})

	t.Run("EnableEventLog", func(t *testing.T) {
		t.Parallel()
		t.Run("when service is not registered", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()

			must.NoError(t, srv.EnableEventlog(), must.Sprint("could not enable eventlog"))
			key, err := registry.OpenKey(registry.LOCAL_MACHINE,
				EVENTLOG_REGISTRY_PATH+`\`+serviceName,
				registry.READ,
			)
			must.NoError(t, err, must.Sprint("registry key should be available"))
			defer key.Close()
			val, _, err := key.GetIntegerValue(EVENTLOG_SUPPORTED_EVENTS_KEY)
			must.NoError(t, err, must.Sprint("registry key value should be available"))
			must.Eq(t, EVENTLOG_SUPPORTED_EVENTS, uint32(val), must.Sprint("registry value should match"))
		})

		t.Run("when service is already registered", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			must.NoError(t, srv.EnableEventlog(), must.Sprint("could not enable eventlog"))
			// Modify value in registry
			key, err := registry.OpenKey(registry.LOCAL_MACHINE,
				EVENTLOG_REGISTRY_PATH+`\`+serviceName,
				registry.ALL_ACCESS,
			)
			err = key.SetDWordValue(EVENTLOG_SUPPORTED_EVENTS_KEY, 1)
			must.NoError(t, err, must.Sprint("could not modify registry value"))

			// Now enable and verify value is correct
			must.NoError(t, srv.EnableEventlog(), must.Sprint("failed to enable eventlog"))
			val, _, err := key.GetIntegerValue(EVENTLOG_SUPPORTED_EVENTS_KEY)
			must.NoError(t, err, must.Sprint("registry value should be available"))
			must.Eq(t, EVENTLOG_SUPPORTED_EVENTS, uint32(val), must.Sprint("registry value should match"))
		})
	})

	t.Run("DisableEventLog", func(t *testing.T) {
		t.Parallel()
		t.Run("when service is not registered", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()

			must.NoError(t, srv.DisableEventlog(), must.Sprint("eventlog disable should not error"))
		})

		t.Run("when service is registered", func(t *testing.T) {
			t.Parallel()
			m, manager, closer := makeManagers()
			defer closer()
			serviceName, closer := generateStubService(m)
			defer closer()
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			must.NoError(t, srv.EnableEventlog(), must.Sprint("eventlog enable should not error"))

			must.NoError(t, srv.DisableEventlog(), must.Sprint("eventlog disable should not error"))
			_, err = registry.OpenKey(registry.LOCAL_MACHINE,
				EVENTLOG_REGISTRY_PATH+`\`+serviceName,
				registry.READ,
			)
			must.ErrorIs(t, err, fs.ErrNotExist, must.Sprint("registry key should no longer exist"))
		})
	})
}

// waits and retries the condition for 10 seconds
func waitForCondition(cond func() bool) bool {
	for range 40 {
		if cond() {
			return true
		}
		time.Sleep(time.Second / 4)
	}

	return false
}

func generateServiceName() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id[:5]
}

func generateStubService(m *mgr.Mgr) (string, func()) {
	id := generateServiceName()
	_, err := m.CreateService(id, `c:\stub`, mgr.Config{})
	if err != nil {
		panic(err)
	}

	return id, func() { deleteStubService(m, id) }
}

func deleteStubService(m *mgr.Mgr, svcId string) {
	srvc, err := m.OpenService(svcId)
	status, err := srvc.Query()
	if status.State != svc.Stopped {
		status, err = srvc.Control(svc.Stop)
		if err != nil {
			panic(err)
		}

		result := waitForCondition(func() bool {
			status, err := srvc.Query()
			if err != nil {
				panic(err)
			}
			return status.State == svc.Stopped
		})
		if !result {
			panic("could not stop service for deletion " + svcId)
		}
	}

	if err != nil {
		panic(err)
	}
	if err := srvc.Delete(); err != nil {
		panic(err)
	}
}

func makeManagers() (*mgr.Mgr, WindowsServiceManager, func()) {
	winM, err := NewWindowsServiceManager()
	if err != nil {
		panic("failed to create service manager")
	}
	m, err := mgr.Connect()
	if err != nil {
		panic("failed to connect to windows mgr")
	}

	return m, winM, func() {
		winM.Close()
		m.Disconnect()
	}
}
