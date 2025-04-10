package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestCoreSelectorSelect(t *testing.T) {
	var (
		totalCores = 46
		maxSpeed   = 100
		coreIds    = make([]uint16, totalCores)
		cores      = make([]numalib.Core, totalCores)
	)
	for i := 1; i < 24; i++ {
		coreIds[i-1] = uint16(i)
		cores[i-1] = numalib.Core{
			SocketID:   0,
			NodeID:     0,
			ID:         hw.CoreID(i),
			Grade:      false,
			Disable:    false,
			BaseSpeed:  0,
			MaxSpeed:   hw.MHz(maxSpeed),
			GuessSpeed: 0,
		}
	}
	for i := 25; i < 48; i++ {
		coreIds[i-2] = uint16(i)
		cores[i-2] = numalib.Core{
			SocketID:   0,
			NodeID:     0,
			ID:         hw.CoreID(i),
			Grade:      false,
			Disable:    false,
			BaseSpeed:  0,
			MaxSpeed:   hw.MHz(maxSpeed),
			GuessSpeed: 0,
		}
	}
	require.Equal(t, coreIds, []uint16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47})

	selector := &coreSelector{
		topology: &numalib.Topology{
			Cores: cores,
		},
		availableCores: idset.From[hw.CoreID](coreIds),
	}

	for _, test := range []struct {
		name        string
		resources   *structs.Resources
		expectedIds []uint16
		expectedMhz hw.MHz
	}{
		{
			name: "request all cores",
			resources: &structs.Resources{
				Cores: totalCores,
			},
			expectedIds: coreIds,
			expectedMhz: hw.MHz(totalCores * maxSpeed),
		},
		{
			name: "request half the cores",
			resources: &structs.Resources{
				Cores: 10,
			},
			expectedIds: coreIds[:10],
			expectedMhz: hw.MHz(10 * maxSpeed),
		},
		{
			name: "request one core",
			resources: &structs.Resources{
				Cores: 1,
			},
			expectedIds: coreIds[:1],
			expectedMhz: hw.MHz(1 * maxSpeed),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids, mhz := selector.Select(test.resources)
			require.Equal(t, test.expectedIds, ids)
			require.Equal(t, test.expectedMhz, mhz)
		})
	}
}
