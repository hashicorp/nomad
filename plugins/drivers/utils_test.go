package drivers

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceUsageRoundTrip(t *testing.T) {
	input := &ResourceUsage{
		CpuStats: &CpuStats{
			SystemMode:       0,
			UserMode:         0.9963907032120152,
			TotalTicks:       21.920595295932515,
			ThrottledPeriods: 2321,
			ThrottledTime:    123,
			Percent:          0.9963906952696598,
			Measured:         []string{"System Mode", "User Mode", "Percent"},
		},
		MemoryStats: &MemoryStats{
			RSS:            25681920,
			Swap:           15681920,
			Usage:          12,
			MaxUsage:       23,
			KernelUsage:    34,
			KernelMaxUsage: 45,
			Measured:       []string{"RSS", "Swap"},
		},
	}

	parsed := resourceUsageFromProto(resourceUsageToProto(input))

	require.EqualValues(t, parsed, input)
}

func TestTaskConfigRoundTrip(t *testing.T) {

	input := &TaskConfig{
		ID:            uuid.Generate(),
		Name:          "task",
		JobName:       "job",
		TaskGroupName: "group",
		Resources: &Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Cpu: structs.AllocatedCpuResources{
					CpuShares: int64(100),
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: int64(300),
				},
			},
			LinuxResources: &LinuxResources{
				MemoryLimitBytes: 300 * 1024 * 1024,
				CPUShares:        100,
				PercentTicks:     float64(100) / float64(3200),
			},
			Ports: &structs.AllocatedPorts{
				{
					Label:  "port",
					Value:  23456,
					To:     8080,
					HostIP: "10.0.0.1",
				},
			},
		},
		Devices: []*DeviceConfig{
			{
				TaskPath:    "task",
				HostPath:    "host",
				Permissions: "perms",
			},
		},
		Mounts: []*MountConfig{
			{
				TaskPath: "task",
				HostPath: "host",
				Readonly: true,
			},
		},
		Env:        map[string]string{"gir": "zim"},
		DeviceEnv:  map[string]string{"foo": "bar"},
		User:       "user",
		AllocDir:   "allocDir",
		StdoutPath: "stdout",
		StderrPath: "stderr",
		AllocID:    uuid.Generate(),
		NetworkIsolation: &NetworkIsolationSpec{
			Mode:   NetIsolationModeGroup,
			Path:   "path",
			Labels: map[string]string{"net": "abc"},
		},
		DNS: &DNSConfig{
			Servers:  []string{"8.8.8.8"},
			Searches: []string{".consul"},
			Options:  []string{"ndots:2"},
		},
	}

	parsed := taskConfigFromProto(taskConfigToProto(input))

	require.EqualValues(t, input, parsed)

}

func Test_networkCreateRequestFromProto(t *testing.T) {
	testCases := []struct {
		inputPB        *proto.CreateNetworkRequest
		expectedOutput *NetworkCreateRequest
		name           string
	}{
		{
			inputPB:        nil,
			expectedOutput: nil,
			name:           "nil safety",
		},
		{
			inputPB: &proto.CreateNetworkRequest{
				AllocId:  "59598b74-86e9-16ee-eb54-24c62935cc7c",
				Hostname: "foobar",
			},
			expectedOutput: &NetworkCreateRequest{
				Hostname: "foobar",
			},
			name: "generic 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := networkCreateRequestFromProto(tc.inputPB)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}
