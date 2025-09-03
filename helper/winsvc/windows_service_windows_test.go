// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"context"
	"io/fs"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestWindowsServiceManager(t *testing.T) {
	ci.Parallel(t)

	t.Run("IsServiceRegistered", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("service does not exist", func(t *testing.T) {
			ci.Parallel(t)
			_, manager := makeManagers(t)

			result, err := manager.IsServiceRegistered("fake-service-name")
			must.NoError(t, err, must.Sprint("check should not error"))
			must.False(t, result, must.Sprint("service should not exist"))
		})

		t.Run("service does exist", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

			result, err := manager.IsServiceRegistered(serviceName)
			must.NoError(t, err, must.Sprint("check should not error"))
			must.True(t, result, must.Sprint("service should exist"))
		})
	})

	t.Run("GetService", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("service does not exist", func(t *testing.T) {
			ci.Parallel(t)
			_, manager := makeManagers(t)
			_, err := manager.GetService("fake-service-name")
			must.ErrorContains(t, err, "specified service does not exist",
				must.Sprint("error should be generated when service does not exist"))
		})

		t.Run("service does exist", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

			srv, err := manager.GetService(serviceName)
			must.NoError(t, err)
			defer srv.Close()
			must.Eq(t, serviceName, srv.Name(), must.Sprint("service name does not match"))
		})
	})

	t.Run("CreateService", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("service does not exist", func(t *testing.T) {
			ci.Parallel(t)
			serviceName := generateServiceName()
			m, manager := makeManagers(t)

			srv, err := manager.CreateService(serviceName, `c:\stub`, WindowsServiceConfiguration{})
			must.NoError(t, err)
			defer srv.Close()
			defer deleteStubService(t, m, serviceName)

			must.Eq(t, serviceName, srv.Name(), must.Sprint("new service name is incorrect"))
		})

		t.Run("service does exist", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

			_, err := manager.CreateService(serviceName, `c:\stub`, WindowsServiceConfiguration{})
			must.ErrorContains(t, err, "service already exists", must.Sprint("service creation should fail"))
		})

		t.Run("with configuration", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateServiceName()
			srv, err := manager.CreateService(serviceName, `c:\stub`,
				WindowsServiceConfiguration{DisplayName: "testing service", StartType: StartDisabled})
			must.NoError(t, err, must.Sprint("service should be created"))
			defer srv.Close()
			defer deleteStubService(t, m, serviceName)

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
	ci.Parallel(t)

	mg, _ := makeManagers(t)
	snmpSvc, err := mg.OpenService(TEST_WINDOWS_SERVICE)
	must.NoError(t, err)
	defer snmpSvc.Close()
	snmpConfig, err := snmpSvc.Config()
	must.NoError(t, err)
	binPath := snmpConfig.BinaryPathName

	t.Run("Name", func(t *testing.T) {
		ci.Parallel(t)
		m, manager := makeManagers(t)
		serviceName := generateStubService(t, m)

		srv, err := manager.GetService(serviceName)
		must.NoError(t, err)
		defer srv.Close()

		must.Eq(t, serviceName, srv.Name(), must.Sprint("service name does not match"))
	})

	t.Run("Configure", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("valid configuration", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

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
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)
			srv, err := manager.GetService(serviceName)

			must.NoError(t, err, must.Sprint("service should be available"))
			err = srv.Configure(WindowsServiceConfiguration{
				DisplayName:    "testing display name",
				BinaryPathName: `c:\stub -with -arguments`,
			})
			must.ErrorContains(t, err, "parameter is incorrect", must.Sprint("invalid configuration should error"))
		})
	})

	t.Run("Start", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("when stopped", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				err = waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped, nil
				})
				must.NoError(t, err, must.Sprint("service must be stopped"))
			}
			must.NoError(t, runnableSvc.Start(), must.Sprint("service should start without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Running, must.Sprint("service should be running"))
		})

		t.Run("when running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				err := waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running, nil
				})
				must.NoError(t, err, must.Sprint("service must be running"))
			}
			must.NoError(t, runnableSvc.Start(), must.Sprint("service should start without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Running, must.Sprint("service should be running"))
		})
	})

	t.Run("Stop", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("when stopped", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				err = waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped, nil
				})
				must.NoError(t, err, must.Sprint("service must be stopped"))
			}
			must.NoError(t, runnableSvc.Stop(), must.Sprint("service should stop without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Stopped, must.Sprint("service should be stopped"))
		})

		t.Run("when running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)

			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				err := waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running, nil
				})
				must.NoError(t, err, must.Sprint("service must be running"))
			}
			must.NoError(t, runnableSvc.Stop(), must.Sprint("service should stop without error"))
			status, err = directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			must.Eq(t, status.State, svc.Stopped, must.Sprint("service should be stopped"))
		})
	})

	t.Run("Delete", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("when service exists", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)

			serviceName := generateStubService(t, m)
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be avaialble"))
			defer srv.Close()

			must.NoError(t, srv.Delete(), must.Sprint("service should be deleted"))
		})

		t.Run("when service does not exist", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)

			serviceName := generateStubService(t, m)
			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be avaialble"))
			defer srv.Close()
			// Delete the service directly
			directSrv, err := m.OpenService(serviceName)
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()
			must.NoError(t, directSrv.Delete(), must.Sprint("service should be deleted"))

			must.ErrorContains(t, srv.Delete(), "marked for deletion",
				must.Sprint("service should have already been deleted"))
		})
	})

	t.Run("IsRunning", func(t *testing.T) {
		ci.Parallel(t)
		t.Run("when service is not running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				err = waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped, nil
				})
				must.NoError(t, err, must.Sprint("service must be stopped"))
			}

			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsRunning()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.False(t, result, must.Sprint("should not show service as running"))
		})

		t.Run("when service is running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				err := waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running, nil
				})
				must.NoError(t, err, must.Sprint("service must be running"))
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
		ci.Parallel(t)
		t.Run("when service is not running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Stopped {
				_, err := directSrv.Control(svc.Stop)
				must.NoError(t, err, must.Sprint("direct stop should not fail"))
				err = waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Stopped, nil
				})
				must.NoError(t, err, must.Sprint("service must be stopped"))
			}

			srv, err := manager.GetService(directSrv.Name)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()
			result, err := srv.IsStopped()
			must.NoError(t, err, must.Sprint("running check should not error"))
			must.True(t, result, must.Sprint("should show service as stopped"))
		})

		t.Run("when service is running", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			runnableSvc := runnableService(t, manager, binPath)
			directSrv, err := m.OpenService(runnableSvc.Name())
			must.NoError(t, err, must.Sprint("direct service should be available"))
			defer directSrv.Close()

			status, err := directSrv.Query()
			must.NoError(t, err, must.Sprint("direct service status should be available"))
			if status.State != svc.Running {
				must.NoError(t, directSrv.Start(), must.Sprint("direct start should not fail"))
				err := waitFor(context.Background(), func() (bool, error) {
					status, err := directSrv.Query()
					must.NoError(t, err, must.Sprint("direct service should be queryable"))
					return status.State == svc.Running, nil
				})
				must.NoError(t, err, must.Sprint("service must be running"))
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
		ci.Parallel(t)
		t.Run("when service is not registered", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

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
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

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
		ci.Parallel(t)
		t.Run("when service is not registered", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

			srv, err := manager.GetService(serviceName)
			must.NoError(t, err, must.Sprint("service should be available"))
			defer srv.Close()

			must.NoError(t, srv.DisableEventlog(), must.Sprint("eventlog disable should not error"))
		})

		t.Run("when service is registered", func(t *testing.T) {
			ci.Parallel(t)
			m, manager := makeManagers(t)
			serviceName := generateStubService(t, m)

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

func generateServiceName() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id[:5]
}

func generateStubService(t *testing.T, m *mgr.Mgr) string {
	t.Helper()

	id := generateServiceName()
	_, err := m.CreateService(id, `c:\stub`, mgr.Config{})
	must.NoError(t, err, must.Sprint("failed to generate stub service"))

	t.Cleanup(func() { deleteStubService(t, m, id) })

	return id
}

func deleteStubService(t *testing.T, m *mgr.Mgr, svcId string) {
	t.Helper()

	srvc, err := m.OpenService(svcId)
	if err != nil {
		// If the service doesn't exist, then deletion is done so not
		// an error. Otherwise, force an error.
		must.ErrorContains(t, err, "service does not exist", must.Sprint("failed to open service"))
		return
	}
	status, err := srvc.Query()
	must.NoError(t, err, must.Sprint("failed to query service"))
	if status.State != svc.Stopped {
		status, err = srvc.Control(svc.Stop)
		must.NoError(t, err, must.Sprint("failed to stop service"))
		err := waitFor(context.Background(), func() (bool, error) {
			status, err := srvc.Query()
			must.NoError(t, err, must.Sprint("failed to query service"))
			return status.State == svc.Stopped, nil
		})
		must.NoError(t, err, must.Sprintf("could not stop service for deletion - %s", svcId))
	}
	if err := srvc.Delete(); err != nil {
		must.ErrorContains(t, err, "service has been marked for deletion", must.Sprint("failed to delete service"))
	}
}

func makeManagers(t *testing.T) (*mgr.Mgr, WindowsServiceManager) {
	t.Helper()

	winM, err := NewWindowsServiceManager()
	must.NoError(t, err, must.Sprint("failed to create service manager"))

	m, err := mgr.Connect()
	must.NoError(t, err, must.Sprint("failed to connect to windows service manager"))

	t.Cleanup(func() {
		winM.Close()
		m.Disconnect()
	})

	return m, winM
}

func runnableService(t *testing.T, m WindowsServiceManager, binPath string) WindowsService {
	t.Helper()

	runnableSvc, err := m.CreateService(generateServiceName(), binPath,
		WindowsServiceConfiguration{StartType: StartManual, BinaryPathName: binPath})
	must.NoError(t, err, must.Sprint("failed to create runnable service"))

	t.Cleanup(func() { runnableSvc.Close() })

	return runnableSvc
}
