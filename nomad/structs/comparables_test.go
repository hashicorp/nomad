// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestComparableDisk_Add(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableDisk
		delta    *ComparableDisk
		expected int64
	}{
		{
			name:     "add positive values",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    &ComparableDisk{DiskMB: 50},
			expected: 150,
		},
		{
			name:     "add zero",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    &ComparableDisk{DiskMB: 0},
			expected: 100,
		},
		{
			name:     "add to zero",
			base:     &ComparableDisk{DiskMB: 0},
			delta:    &ComparableDisk{DiskMB: 50},
			expected: 50,
		},
		{
			name:     "add nil delta",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    nil,
			expected: 100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Add(tc.delta)
			must.Eq(t, tc.expected, tc.base.DiskMB)
		})
	}
}

func TestComparableDisk_Subtract(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableDisk
		delta    *ComparableDisk
		expected int64
	}{
		{
			name:     "subtract positive values",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    &ComparableDisk{DiskMB: 50},
			expected: 50,
		},
		{
			name:     "subtract zero",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    &ComparableDisk{DiskMB: 0},
			expected: 100,
		},
		{
			name:     "subtract from zero",
			base:     &ComparableDisk{DiskMB: 0},
			delta:    &ComparableDisk{DiskMB: 50},
			expected: -50,
		},
		{
			name:     "subtract nil delta",
			base:     &ComparableDisk{DiskMB: 100},
			delta:    nil,
			expected: 100,
		},
		{
			name:     "subtract larger value",
			base:     &ComparableDisk{DiskMB: 50},
			delta:    &ComparableDisk{DiskMB: 100},
			expected: -50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Subtract(tc.delta)
			must.Eq(t, tc.expected, tc.base.DiskMB)
		})
	}
}

func TestComparableDisk_Superset(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableDisk
		other    *ComparableDisk
		expected bool
	}{
		{
			name:     "base is superset",
			base:     &ComparableDisk{DiskMB: 100},
			other:    &ComparableDisk{DiskMB: 50},
			expected: true,
		},
		{
			name:     "base equals other",
			base:     &ComparableDisk{DiskMB: 100},
			other:    &ComparableDisk{DiskMB: 100},
			expected: true,
		},
		{
			name:     "base is not superset",
			base:     &ComparableDisk{DiskMB: 50},
			other:    &ComparableDisk{DiskMB: 100},
			expected: false,
		},
		{
			name:     "other is nil",
			base:     &ComparableDisk{DiskMB: 100},
			other:    nil,
			expected: false,
		},
		{
			name:     "both zero",
			base:     &ComparableDisk{DiskMB: 0},
			other:    &ComparableDisk{DiskMB: 0},
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.base.Superset(tc.other)
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestComparableMem_Add(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableMem
		delta    *ComparableMem
		expected int64
	}{
		{
			name:     "add positive values",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
			expected: 1536,
		},
		{
			name:     "add zero",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 0}},
			expected: 1024,
		},
		{
			name:     "add nil delta",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    nil,
			expected: 1024,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Add(tc.delta)
			must.Eq(t, tc.expected, tc.base.MemoryMB)
		})
	}
}

func TestComparableMem_Subtract(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableMem
		delta    *ComparableMem
		expected int64
	}{
		{
			name:     "subtract positive values",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
			expected: 512,
		},
		{
			name:     "subtract zero",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 0}},
			expected: 1024,
		},
		{
			name:     "subtract nil delta",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			delta:    nil,
			expected: 1024,
		},
		{
			name:     "subtract larger value",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
			delta:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			expected: -512,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Subtract(tc.delta)
			must.Eq(t, tc.expected, tc.base.MemoryMB)
		})
	}
}

func TestComparableMem_Superset(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableMem
		other    *ComparableMem
		expected bool
	}{
		{
			name:     "base is superset",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			other:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
			expected: true,
		},
		{
			name:     "base equals other",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			other:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			expected: true,
		},
		{
			name:     "base is not superset",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
			other:    &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			expected: false,
		},
		{
			name:     "other is nil",
			base:     &ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
			other:    nil,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.base.Superset(tc.other)
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestComparableCPU_Add(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableCPU
		delta    *ComparableCPU
		expected int64
	}{
		{
			name:     "add positive values",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
			expected: 1500,
		},
		{
			name:     "add zero",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 0}},
			expected: 1000,
		},
		{
			name:     "add nil delta",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    nil,
			expected: 1000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Add(tc.delta)
			must.Eq(t, tc.expected, tc.base.CpuShares)
		})
	}
}

func TestComparableCPU_Subtract(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableCPU
		delta    *ComparableCPU
		expected int64
	}{
		{
			name:     "subtract positive values",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
			expected: 500,
		},
		{
			name:     "subtract zero",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 0}},
			expected: 1000,
		},
		{
			name:     "subtract nil delta",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			delta:    nil,
			expected: 1000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Subtract(tc.delta)
			must.Eq(t, tc.expected, tc.base.CpuShares)
		})
	}
}

func TestComparableCPU_Superset(t *testing.T) {
	cases := []struct {
		name     string
		base     *ComparableCPU
		other    *ComparableCPU
		expected bool
	}{
		{
			name:     "base is superset by cpu shares",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			other:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
			expected: true,
		},
		{
			name:     "base equals other",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			other:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			expected: true,
		},
		{
			name:     "base is not superset by cpu shares",
			base:     &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
			other:    &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
			expected: false,
		},
		{
			name: "base is superset with reserved cores",
			base: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares:     1000,
				ReservedCores: []uint16{0, 1, 2},
			}},
			other: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares:     500,
				ReservedCores: []uint16{0, 1},
			}},
			expected: true,
		},
		{
			name: "base is not superset with reserved cores",
			base: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares:     1000,
				ReservedCores: []uint16{0, 1},
			}},
			other: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares:     500,
				ReservedCores: []uint16{0, 1, 2},
			}},
			expected: false,
		},
		{
			name: "base has no reserved cores but other does",
			base: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares: 1000,
			}},
			other: &ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
				CpuShares:     500,
				ReservedCores: []uint16{0, 1},
			}},
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.base.Superset(tc.other)
			must.Eq(t, tc.expected, result)
		})
	}
}

func TestBaseComparableResource_Add(t *testing.T) {
	cases := []struct {
		name          string
		base          *BaseComparableResource
		delta         *BaseComparableResource
		expectedCPU   int64
		expectedMem   int64
		expectedDisk  int64
	}{
		{
			name: "add all resources",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			delta: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			expectedCPU:  1500,
			expectedMem:  1536,
			expectedDisk: 3072,
		},
		{
			name: "add nil delta",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			delta:        nil,
			expectedCPU:  1000,
			expectedMem:  1024,
			expectedDisk: 2048,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Add(tc.delta)
			must.Eq(t, tc.expectedCPU, tc.base.ComparableCPU.CpuShares)
			must.Eq(t, tc.expectedMem, tc.base.ComparableMem.MemoryMB)
			must.Eq(t, tc.expectedDisk, tc.base.ComparableDisk.DiskMB)
		})
	}
}

func TestBaseComparableResource_Subtract(t *testing.T) {
	cases := []struct {
		name          string
		base          *BaseComparableResource
		delta         *BaseComparableResource
		expectedCPU   int64
		expectedMem   int64
		expectedDisk  int64
	}{
		{
			name: "subtract all resources",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			delta: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			expectedCPU:  500,
			expectedMem:  512,
			expectedDisk: 1024,
		},
		{
			name: "subtract nil delta",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			delta:        nil,
			expectedCPU:  1000,
			expectedMem:  1024,
			expectedDisk: 2048,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Subtract(tc.delta)
			must.Eq(t, tc.expectedCPU, tc.base.ComparableCPU.CpuShares)
			must.Eq(t, tc.expectedMem, tc.base.ComparableMem.MemoryMB)
			must.Eq(t, tc.expectedDisk, tc.base.ComparableDisk.DiskMB)
		})
	}
}

func TestBaseComparableResource_Superset(t *testing.T) {
	cases := []struct {
		name           string
		base           *BaseComparableResource
		other          *BaseComparableResource
		expectedResult bool
		expectedReason string
	}{
		{
			name: "base is superset of all resources",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			other: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			expectedResult: true,
			expectedReason: "",
		},
		{
			name: "base fails cpu superset",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			other: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			expectedResult: false,
			expectedReason: "cpu",
		},
		{
			name: "base fails mem superset",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			other: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			expectedResult: false,
			expectedReason: "mem",
		},
		{
			name: "base fails disk superset",
			base: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 1000}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
				ComparableDisk: ComparableDisk{DiskMB: 1024},
			},
			other: &BaseComparableResource{
				ComparableCPU:  ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{CpuShares: 500}},
				ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 512}},
				ComparableDisk: ComparableDisk{DiskMB: 2048},
			},
			expectedResult: false,
			expectedReason: "disk",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, reason := tc.base.Superset(tc.other)
			must.Eq(t, tc.expectedResult, result)
			must.Eq(t, tc.expectedReason, reason)
		})
	}
}

func TestBaseComparableResource_Copy(t *testing.T) {
	original := &BaseComparableResource{
		ComparableCPU: ComparableCPU{AllocatedCpuResources: AllocatedCpuResources{
			CpuShares:     1000,
			ReservedCores: []uint16{0, 1},
		}},
		ComparableMem:  ComparableMem{AllocatedMemoryResources: AllocatedMemoryResources{MemoryMB: 1024}},
		ComparableDisk: ComparableDisk{DiskMB: 2048},
	}

	copied := original.Copy()

	// Verify values are equal
	must.Eq(t, original.ComparableCPU.CpuShares, copied.ComparableCPU.CpuShares)
	must.Eq(t, original.ComparableMem.MemoryMB, copied.ComparableMem.MemoryMB)
	must.Eq(t, original.ComparableDisk.DiskMB, copied.ComparableDisk.DiskMB)

	// Modify copy and verify original is unchanged
	copied.ComparableCPU.CpuShares = 2000
	copied.ComparableMem.MemoryMB = 2048
	copied.ComparableDisk.DiskMB = 4096

	must.Eq(t, int64(1000), original.ComparableCPU.CpuShares)
	must.Eq(t, int64(1024), original.ComparableMem.MemoryMB)
	must.Eq(t, int64(2048), original.ComparableDisk.DiskMB)
}

func TestComparableNetworks_Copy(t *testing.T) {
	original := &ComparableNetworks{
		FlattenedNetworks: Networks{
			&NetworkResource{
				Device: "eth0",
				IP:     "192.168.1.1",
			},
		},
		SharedNetworks: Networks{
			&NetworkResource{
				Device: "eth1",
				IP:     "10.0.0.1",
			},
		},
		SharedPorts: AllocatedPorts{
			{
				Label: "http",
				Value: 8080,
			},
		},
	}

	copied := original.Copy()

	// Verify values are equal
	must.Eq(t, len(original.FlattenedNetworks), len(copied.FlattenedNetworks))
	must.Eq(t, len(original.SharedNetworks), len(copied.SharedNetworks))
	must.Eq(t, len(original.SharedPorts), len(copied.SharedPorts))

	// Verify it's a deep copy by modifying the copy
	copied.FlattenedNetworks[0].IP = "192.168.1.2"
	must.Eq(t, "192.168.1.1", original.FlattenedNetworks[0].IP)
	must.Eq(t, "192.168.1.2", copied.FlattenedNetworks[0].IP)
}
