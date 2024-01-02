// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package wrapper

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func Test_NewHandlerWrapper(t *testing.T) {
	log := hclog.NewNullLogger()
	mockProvider := regMock.NewServiceRegistrationHandler(log)
	wrapper := NewHandlerWrapper(log, mockProvider, mockProvider)
	require.NotNil(t, wrapper)
	require.NotNil(t, wrapper.log)
	require.NotNil(t, wrapper.nomadServiceProvider)
	require.NotNil(t, wrapper.consulServiceProvider)
}

func TestHandlerWrapper_RegisterWorkload(t *testing.T) {
	testCases := []struct {
		testFn func(t *testing.T)
		name   string
	}{
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Call the function with no services and check that nothing is
				// registered.
				require.NoError(t, wrapper.RegisterWorkload(&serviceregistration.WorkloadServices{}))
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 0)
			},
			name: "zero services",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with an unknown provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: "istio",
						},
					},
				}

				// Call register and ensure an error is returned along with
				// nothing registered in the providers.
				err := wrapper.RegisterWorkload(&workload)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unknown service registration provider: \"istio\"")
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 0)

			},
			name: "unknown provider",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with the nomad provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderNomad,
						},
					},
				}

				// Call register and ensure no error is returned along with the
				// correct operations.
				require.NoError(t, wrapper.RegisterWorkload(&workload))
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 1)

			},
			name: "nomad provider",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with the consul provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderConsul,
						},
					},
				}

				// Call register and ensure no error is returned along with the
				// correct operations.
				require.NoError(t, wrapper.RegisterWorkload(&workload))
				require.Len(t, consul.GetOps(), 1)
				require.Len(t, nomad.GetOps(), 0)
			},
			name: "consul provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(t)
		})
	}
}

func TestHandlerWrapper_RemoveWorkload(t *testing.T) {
	testCases := []struct {
		testFn func(t *testing.T)
		name   string
	}{
		{
			testFn: func(t *testing.T) {
				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Call the function with no services and check that consul is
				// defaulted to.
				wrapper.RemoveWorkload(&serviceregistration.WorkloadServices{})
				require.Len(t, consul.GetOps(), 1)
				require.Len(t, nomad.GetOps(), 0)
			},
			name: "zero services",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with an unknown provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: "istio",
						},
					},
				}

				// Call remove and ensure nothing registered in the providers.
				wrapper.RemoveWorkload(&workload)
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 0)
			},
			name: "unknown provider",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with the consul provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderConsul,
						},
					},
				}

				// Call remove and ensure the correct backend includes
				// operations.
				wrapper.RemoveWorkload(&workload)
				require.Len(t, consul.GetOps(), 1)
				require.Len(t, nomad.GetOps(), 0)
			},
			name: "consul provider",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Generate a minimal workload with the nomad provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderNomad,
						},
					},
				}

				// Call remove and ensure the correct backend includes
				// operations.
				wrapper.RemoveWorkload(&workload)
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 1)
			},
			name: "nomad provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(t)
		})
	}
}

func TestHandlerWrapper_UpdateWorkload(t *testing.T) {
	testCases := []struct {
		testFn func(t *testing.T)
		name   string
	}{
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Call the function with no services and check that nothing is
				// registered in either mock backend.
				err := wrapper.UpdateWorkload(&serviceregistration.WorkloadServices{},
					&serviceregistration.WorkloadServices{})
				require.NoError(t, err)
				require.Len(t, consul.GetOps(), 0)
				require.Len(t, nomad.GetOps(), 0)

			},
			name: "zero new or old",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Create a single workload that we can use twice, using the
				// consul provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderConsul,
						},
					},
				}

				// Call the function and ensure the consul backend has the
				// expected operations.
				require.NoError(t, wrapper.UpdateWorkload(&workload, &workload))
				require.Len(t, nomad.GetOps(), 0)

				consulOps := consul.GetOps()
				require.Len(t, consulOps, 1)
				require.Equal(t, "update", consulOps[0].Op)
			},
			name: "consul new and old",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Create a single workload that we can use twice, using the
				// nomad provider.
				workload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderNomad,
						},
					},
				}

				// Call the function and ensure the nomad backend has the
				// expected operations.
				require.NoError(t, wrapper.UpdateWorkload(&workload, &workload))
				require.Len(t, consul.GetOps(), 0)

				nomadOps := nomad.GetOps()
				require.Len(t, nomadOps, 1)
				require.Equal(t, "update", nomadOps[0].Op)
			},
			name: "nomad new and old",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Create each workload.
				newWorkload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderNomad,
						},
					},
				}

				oldWorkload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderConsul,
						},
					},
				}

				// Call the function and ensure the backends have the expected
				// operations.
				require.NoError(t, wrapper.UpdateWorkload(&oldWorkload, &newWorkload))

				nomadOps := nomad.GetOps()
				require.Len(t, nomadOps, 1)
				require.Equal(t, "add", nomadOps[0].Op)

				consulOps := consul.GetOps()
				require.Len(t, consulOps, 1)
				require.Equal(t, "remove", consulOps[0].Op)
			},
			name: "nomad new and consul old",
		},
		{
			testFn: func(t *testing.T) {

				// Generate the test wrapper and provider mocks.
				wrapper, consul, nomad := setupTestWrapper()

				// Create each workload.
				newWorkload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderConsul,
						},
					},
				}

				oldWorkload := serviceregistration.WorkloadServices{
					Services: []*structs.Service{
						{
							Provider: structs.ServiceProviderNomad,
						},
					},
				}

				// Call the function and ensure the backends have the expected
				// operations.
				require.NoError(t, wrapper.UpdateWorkload(&oldWorkload, &newWorkload))

				nomadOps := nomad.GetOps()
				require.Len(t, nomadOps, 1)
				require.Equal(t, "remove", nomadOps[0].Op)

				consulOps := consul.GetOps()
				require.Len(t, consulOps, 1)
				require.Equal(t, "add", consulOps[0].Op)
			},
			name: "consul new and nomad old",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(t)
		})
	}
}

func setupTestWrapper() (*HandlerWrapper, *regMock.ServiceRegistrationHandler, *regMock.ServiceRegistrationHandler) {
	log := hclog.NewNullLogger()
	consulMock := regMock.NewServiceRegistrationHandler(log)
	nomadMock := regMock.NewServiceRegistrationHandler(log)
	wrapper := NewHandlerWrapper(log, consulMock, nomadMock)
	return wrapper, consulMock, nomadMock
}
