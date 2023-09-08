// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestAllocRegistration_Copy(t *testing.T) {
	testCases := []struct {
		inputAllocRegistration *AllocRegistration
		name                   string
	}{
		{
			inputAllocRegistration: &AllocRegistration{
				Tasks: map[string]*ServiceRegistrations{},
			},
			name: "empty tasks map",
		},
		{
			inputAllocRegistration: &AllocRegistration{
				Tasks: map[string]*ServiceRegistrations{
					"cache": {
						Services: map[string]*ServiceRegistration{
							"redis-db": {
								ServiceID: "service-id-1",
								CheckIDs: map[string]struct{}{
									"check-id-1": {},
									"check-id-2": {},
									"check-id-3": {},
								},
								CheckOnUpdate: map[string]string{
									"check-id-1": structs.OnUpdateIgnore,
									"check-id-2": structs.OnUpdateRequireHealthy,
									"check-id-3": structs.OnUpdateIgnoreWarn,
								},
							},
						},
					},
				},
			},
			name: "non-empty tasks map",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputAllocRegistration.Copy()
			require.Equal(t, tc.inputAllocRegistration, actualOutput)
		})
	}
}
