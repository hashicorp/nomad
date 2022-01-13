package structs

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveAllocs(t *testing.T) {
	l := []*Allocation{
		{ID: "foo"},
		{ID: "bar"},
		{ID: "baz"},
		{ID: "zip"},
	}

	out := RemoveAllocs(l, []*Allocation{l[1], l[3]})
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	if out[0].ID != "foo" && out[1].ID != "baz" {
		t.Fatalf("bad: %#v", out)
	}
}

func TestFilterTerminalAllocs(t *testing.T) {
	l := []*Allocation{
		{
			ID:            "bar",
			Name:          "myname1",
			DesiredStatus: AllocDesiredStatusEvict,
		},
		{ID: "baz", DesiredStatus: AllocDesiredStatusStop},
		{
			ID:            "foo",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusPending,
		},
		{
			ID:            "bam",
			Name:          "myname",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusComplete,
			CreateIndex:   5,
		},
		{
			ID:            "lol",
			Name:          "myname",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusComplete,
			CreateIndex:   2,
		},
	}

	out, terminalAllocs := FilterTerminalAllocs(l)
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}
	if out[0].ID != "foo" {
		t.Fatalf("bad: %#v", out)
	}

	if len(terminalAllocs) != 3 {
		for _, o := range terminalAllocs {
			fmt.Printf("%#v \n", o)
		}

		t.Fatalf("bad: %#v", terminalAllocs)
	}

	if terminalAllocs["myname"].ID != "bam" {
		t.Fatalf("bad: %#v", terminalAllocs["myname"])
	}
}

// COMPAT(0.11): Remove in 0.11
func TestAllocsFit_PortsOvercommitted_Old(t *testing.T) {
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
		},
	}

	a1 := &Allocation{
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name:          "web",
					EphemeralDisk: DefaultEphemeralDisk(),
				},
			},
		},
		TaskResources: map[string]*Resources{
			"web": {
				Networks: []*NetworkResource{
					{
						Device:        "eth0",
						IP:            "10.0.0.1",
						MBits:         50,
						ReservedPorts: []Port{{"main", 8000, 80, ""}},
					},
				},
			},
		},
	}

	// Should fit one allocation
	fit, dim, _, err := AllocsFit(n, []*Allocation{a1}, nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("Bad: %s", dim)
	}

	// Should not fit second allocation
	fit, _, _, err = AllocsFit(n, []*Allocation{a1, a1}, nil, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("Bad")
	}
}

// COMPAT(0.11): Remove in 0.11
func TestAllocsFit_Old(t *testing.T) {
	require := require.New(t)

	n := &Node{
		Resources: &Resources{
			CPU:      2000,
			MemoryMB: 2048,
			DiskMB:   10000,
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
		},
		Reserved: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 80, 0, ""}},
				},
			},
		},
	}

	a1 := &Allocation{
		Resources: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 8000, 80, ""}},
				},
			},
		},
	}

	// Should fit one allocation
	fit, _, used, err := AllocsFit(n, []*Allocation{a1}, nil, false)
	require.NoError(err)
	require.True(fit)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)

	// Should not fit second allocation
	fit, _, used, err = AllocsFit(n, []*Allocation{a1, a1}, nil, false)
	require.NoError(err)
	require.False(fit)

	// Sanity check the used resources
	require.EqualValues(2000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(2048, used.Flattened.Memory.MemoryMB)
}

// COMPAT(0.11): Remove in 0.11
func TestAllocsFit_TerminalAlloc_Old(t *testing.T) {
	require := require.New(t)

	n := &Node{
		Resources: &Resources{
			CPU:      2000,
			MemoryMB: 2048,
			DiskMB:   10000,
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
		},
		Reserved: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 80, 0, ""}},
				},
			},
		},
	}

	a1 := &Allocation{
		Resources: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 8000, 0, ""}},
				},
			},
		},
	}

	// Should fit one allocation
	fit, _, used, err := AllocsFit(n, []*Allocation{a1}, nil, false)
	require.NoError(err)
	require.True(fit)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)

	// Should fit second allocation since it is terminal
	a2 := a1.Copy()
	a2.DesiredStatus = AllocDesiredStatusStop
	fit, _, used, err = AllocsFit(n, []*Allocation{a1, a2}, nil, false)
	require.NoError(err)
	require.True(fit)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)
}

func TestAllocsFit(t *testing.T) {
	require := require.New(t)

	n := &Node{
		NodeResources: &NodeResources{
			Cpu: NodeCpuResources{
				CpuShares: 2000,
			},
			Memory: NodeMemoryResources{
				MemoryMB: 2048,
			},
			Disk: NodeDiskResources{
				DiskMB: 10000,
			},
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
			NodeNetworks: []*NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Addresses: []NodeNetworkAddress{
						{
							Address: "10.0.0.1",
						},
					},
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Cpu: NodeReservedCpuResources{
				CpuShares: 1000,
			},
			Memory: NodeReservedMemoryResources{
				MemoryMB: 1024,
			},
			Disk: NodeReservedDiskResources{
				DiskMB: 5000,
			},
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "80",
			},
		},
	}

	a1 := &Allocation{
		AllocatedResources: &AllocatedResources{
			Tasks: map[string]*AllocatedTaskResources{
				"web": {
					Cpu: AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
			Shared: AllocatedSharedResources{
				DiskMB: 5000,
				Networks: Networks{
					{
						Mode:          "host",
						IP:            "10.0.0.1",
						ReservedPorts: []Port{{"main", 8000, 0, ""}},
					},
				},
				Ports: AllocatedPorts{
					{
						Label:  "main",
						Value:  8000,
						HostIP: "10.0.0.1",
					},
				},
			},
		},
	}

	// Should fit one allocation
	fit, _, used, err := AllocsFit(n, []*Allocation{a1}, nil, false)
	require.NoError(err)
	require.True(fit)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)

	// Should not fit second allocation
	fit, _, used, err = AllocsFit(n, []*Allocation{a1, a1}, nil, false)
	require.NoError(err)
	require.False(fit)

	// Sanity check the used resources
	require.EqualValues(2000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(2048, used.Flattened.Memory.MemoryMB)
}

func TestAllocsFit_TerminalAlloc(t *testing.T) {
	require := require.New(t)

	n := &Node{
		NodeResources: &NodeResources{
			Cpu: NodeCpuResources{
				CpuShares: 2000,
			},
			Memory: NodeMemoryResources{
				MemoryMB: 2048,
			},
			Disk: NodeDiskResources{
				DiskMB: 10000,
			},
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					IP:     "10.0.0.1",
					MBits:  100,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Cpu: NodeReservedCpuResources{
				CpuShares: 1000,
			},
			Memory: NodeReservedMemoryResources{
				MemoryMB: 1024,
			},
			Disk: NodeReservedDiskResources{
				DiskMB: 5000,
			},
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "80",
			},
		},
	}

	a1 := &Allocation{
		AllocatedResources: &AllocatedResources{
			Tasks: map[string]*AllocatedTaskResources{
				"web": {
					Cpu: AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: AllocatedMemoryResources{
						MemoryMB: 1024,
					},
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "10.0.0.1",
							MBits:         50,
							ReservedPorts: []Port{{"main", 8000, 80, ""}},
						},
					},
				},
			},
			Shared: AllocatedSharedResources{
				DiskMB: 5000,
			},
		},
	}

	// Should fit one allocation
	fit, _, used, err := AllocsFit(n, []*Allocation{a1}, nil, false)
	require.NoError(err)
	require.True(fit)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)

	// Should fit second allocation since it is terminal
	a2 := a1.Copy()
	a2.DesiredStatus = AllocDesiredStatusStop
	fit, dim, used, err := AllocsFit(n, []*Allocation{a1, a2}, nil, false)
	require.NoError(err)
	require.True(fit, dim)

	// Sanity check the used resources
	require.EqualValues(1000, used.Flattened.Cpu.CpuShares)
	require.EqualValues(1024, used.Flattened.Memory.MemoryMB)
}

// Tests that AllocsFit detects device collisions
func TestAllocsFit_Devices(t *testing.T) {
	require := require.New(t)

	n := MockNvidiaNode()
	a1 := &Allocation{
		AllocatedResources: &AllocatedResources{
			Tasks: map[string]*AllocatedTaskResources{
				"web": {
					Cpu: AllocatedCpuResources{
						CpuShares: 1000,
					},
					Memory: AllocatedMemoryResources{
						MemoryMB: 1024,
					},
					Devices: []*AllocatedDeviceResource{
						{
							Type:      "gpu",
							Vendor:    "nvidia",
							Name:      "1080ti",
							DeviceIDs: []string{n.NodeResources.Devices[0].Instances[0].ID},
						},
					},
				},
			},
			Shared: AllocatedSharedResources{
				DiskMB: 5000,
			},
		},
	}
	a2 := a1.Copy()
	a2.AllocatedResources.Tasks["web"] = &AllocatedTaskResources{
		Cpu: AllocatedCpuResources{
			CpuShares: 1000,
		},
		Memory: AllocatedMemoryResources{
			MemoryMB: 1024,
		},
		Devices: []*AllocatedDeviceResource{
			{
				Type:      "gpu",
				Vendor:    "nvidia",
				Name:      "1080ti",
				DeviceIDs: []string{n.NodeResources.Devices[0].Instances[0].ID}, // Use the same ID
			},
		},
	}

	// Should fit one allocation
	fit, _, _, err := AllocsFit(n, []*Allocation{a1}, nil, true)
	require.NoError(err)
	require.True(fit)

	// Should not fit second allocation
	fit, msg, _, err := AllocsFit(n, []*Allocation{a1, a2}, nil, true)
	require.NoError(err)
	require.False(fit)
	require.Equal("device oversubscribed", msg)

	// Should not fit second allocation but won't detect since we disabled
	// devices
	fit, _, _, err = AllocsFit(n, []*Allocation{a1, a2}, nil, false)
	require.NoError(err)
	require.True(fit)
}

// COMPAT(0.11): Remove in 0.11
func TestScoreFitBinPack_Old(t *testing.T) {
	node := &Node{}
	node.Resources = &Resources{
		CPU:      4096,
		MemoryMB: 8192,
	}
	node.Reserved = &Resources{
		CPU:      2048,
		MemoryMB: 4096,
	}

	// Test a perfect fit
	util := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 2048,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 4096,
			},
		},
	}
	score := ScoreFitBinPack(node, util)
	if score != 18.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test the worst fit
	util = &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 0,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 0,
			},
		},
	}
	score = ScoreFitBinPack(node, util)
	if score != 0.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test a mid-case scenario
	util = &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 1024,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 2048,
			},
		},
	}
	score = ScoreFitBinPack(node, util)
	if score < 10.0 || score > 16.0 {
		t.Fatalf("bad: %v", score)
	}
}

func TestScoreFitBinPack(t *testing.T) {
	node := &Node{}
	node.NodeResources = &NodeResources{
		Cpu: NodeCpuResources{
			CpuShares: 4096,
		},
		Memory: NodeMemoryResources{
			MemoryMB: 8192,
		},
	}
	node.ReservedResources = &NodeReservedResources{
		Cpu: NodeReservedCpuResources{
			CpuShares: 2048,
		},
		Memory: NodeReservedMemoryResources{
			MemoryMB: 4096,
		},
	}

	cases := []struct {
		name         string
		flattened    AllocatedTaskResources
		binPackScore float64
		spreadScore  float64
	}{
		{
			name: "almost filled node, but with just enough hole",
			flattened: AllocatedTaskResources{
				Cpu:    AllocatedCpuResources{CpuShares: 2048},
				Memory: AllocatedMemoryResources{MemoryMB: 4096},
			},
			binPackScore: 18,
			spreadScore:  0,
		},
		{
			name: "unutilized node",
			flattened: AllocatedTaskResources{
				Cpu:    AllocatedCpuResources{CpuShares: 0},
				Memory: AllocatedMemoryResources{MemoryMB: 0},
			},
			binPackScore: 0,
			spreadScore:  18,
		},
		{
			name: "mid-case scnario",
			flattened: AllocatedTaskResources{
				Cpu:    AllocatedCpuResources{CpuShares: 1024},
				Memory: AllocatedMemoryResources{MemoryMB: 2048},
			},
			binPackScore: 13.675,
			spreadScore:  4.325,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			util := &ComparableResources{Flattened: c.flattened}

			binPackScore := ScoreFitBinPack(node, util)
			require.InDelta(t, c.binPackScore, binPackScore, 0.001, "binpack score")

			spreadScore := ScoreFitSpread(node, util)
			require.InDelta(t, c.spreadScore, spreadScore, 0.001, "spread score")

			require.InDelta(t, 18, binPackScore+spreadScore, 0.001, "score sum")
		})
	}
}

func TestACLPolicyListHash(t *testing.T) {
	h1 := ACLPolicyListHash(nil)
	assert.NotEqual(t, "", h1)

	p1 := &ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", uuid.Generate()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}

	h2 := ACLPolicyListHash([]*ACLPolicy{p1})
	assert.NotEqual(t, "", h2)
	assert.NotEqual(t, h1, h2)

	// Create P2 as copy of P1 with new name
	p2 := &ACLPolicy{}
	*p2 = *p1
	p2.Name = fmt.Sprintf("policy-%s", uuid.Generate())

	h3 := ACLPolicyListHash([]*ACLPolicy{p1, p2})
	assert.NotEqual(t, "", h3)
	assert.NotEqual(t, h2, h3)

	h4 := ACLPolicyListHash([]*ACLPolicy{p2})
	assert.NotEqual(t, "", h4)
	assert.NotEqual(t, h3, h4)

	// ModifyIndex should change the hash
	p2.ModifyIndex++
	h5 := ACLPolicyListHash([]*ACLPolicy{p2})
	assert.NotEqual(t, "", h5)
	assert.NotEqual(t, h4, h5)
}

func TestCompileACLObject(t *testing.T) {
	p1 := &ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", uuid.Generate()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}

	// Create P2 as copy of P1 with new name
	p2 := &ACLPolicy{}
	*p2 = *p1
	p2.Name = fmt.Sprintf("policy-%s", uuid.Generate())

	// Create a small cache
	cache, err := lru.New2Q(16)
	assert.Nil(t, err)

	// Test compilation
	aclObj, err := CompileACLObject(cache, []*ACLPolicy{p1})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj)

	// Should get the same object
	aclObj2, err := CompileACLObject(cache, []*ACLPolicy{p1})
	assert.Nil(t, err)
	if aclObj != aclObj2 {
		t.Fatalf("expected the same object")
	}

	// Should get another object
	aclObj3, err := CompileACLObject(cache, []*ACLPolicy{p1, p2})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj3)
	if aclObj == aclObj3 {
		t.Fatalf("unexpected same object")
	}

	// Should be order independent
	aclObj4, err := CompileACLObject(cache, []*ACLPolicy{p2, p1})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj4)
	if aclObj3 != aclObj4 {
		t.Fatalf("expected same object")
	}
}

// TestGenerateMigrateToken asserts the migrate token is valid for use in HTTP
// headers and CompareMigrateToken works as expected.
func TestGenerateMigrateToken(t *testing.T) {
	assert := assert.New(t)
	allocID := uuid.Generate()
	nodeSecret := uuid.Generate()
	token, err := GenerateMigrateToken(allocID, nodeSecret)
	assert.Nil(err)
	_, err = base64.URLEncoding.DecodeString(token)
	assert.Nil(err)

	assert.True(CompareMigrateToken(allocID, nodeSecret, token))
	assert.False(CompareMigrateToken("x", nodeSecret, token))
	assert.False(CompareMigrateToken(allocID, "x", token))
	assert.False(CompareMigrateToken(allocID, nodeSecret, "x"))

	token2, err := GenerateMigrateToken("x", nodeSecret)
	assert.Nil(err)
	assert.False(CompareMigrateToken(allocID, nodeSecret, token2))
	assert.True(CompareMigrateToken("x", nodeSecret, token2))
}

func TestMergeMultierrorWarnings(t *testing.T) {
	var errs []error

	// empty
	str := MergeMultierrorWarnings(errs...)
	require.Equal(t, "", str)

	// non-empty
	errs = []error{
		errors.New("foo"),
		nil,
		errors.New("bar"),
	}

	str = MergeMultierrorWarnings(errs...)

	require.Equal(t, "2 warning(s):\n\n* foo\n* bar", str)
}

// TestParsePortRanges asserts ParsePortRanges errors on invalid port ranges.
func TestParsePortRanges(t *testing.T) {
	cases := []struct {
		name string
		spec string
		err  string
	}{
		{
			name: "UnmatchedDash",
			spec: "-1",
			err:  `strconv.ParseUint: parsing "": invalid syntax`,
		},
		{
			name: "Zero",
			spec: "0",
			err:  "port must be > 0",
		},
		{
			name: "TooBig",
			spec: fmt.Sprintf("1-%d", maxValidPort+1),
			err:  "port must be < 65536 but found 65537",
		},
		{
			name: "WayTooBig",           // would OOM if not caught early enough
			spec: "9223372036854775807", // (2**63)-1
			err:  "port must be < 65536 but found 9223372036854775807",
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			results, err := ParsePortRanges(tc.spec)
			require.Nil(t, results)
			require.EqualError(t, err, tc.err)
		})
	}
}
