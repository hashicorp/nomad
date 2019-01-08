// +build ent

package structs

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelPolicySetHash(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	out1 := sp.SetHash()
	assert.NotNil(t, out1)
	assert.NotNil(t, sp.Hash)
	assert.Equal(t, out1, sp.Hash)

	sp.Policy = "main = rule { false }"
	out2 := sp.SetHash()
	assert.NotNil(t, out2)
	assert.NotNil(t, sp.Hash)
	assert.Equal(t, out2, sp.Hash)
	assert.NotEqual(t, out1, out2)
}

func TestSentinelPolicy_Validate(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	// Test a good policy
	assert.Nil(t, sp.Validate())

	// Try an invalid name
	sp.Name = "hi@there"
	assert.NotNil(t, sp.Validate())

	// Try an invalid description
	sp.Name = "test"
	sp.Description = string(make([]byte, 1000))
	assert.NotNil(t, sp.Validate())

	// Try an invalid scope
	sp.Description = ""
	sp.Scope = "random"
	assert.NotNil(t, sp.Validate())

	// Try an invalid type
	sp.Scope = SentinelScopeSubmitJob
	sp.EnforcementLevel = "yolo"
	assert.NotNil(t, sp.Validate())

	// Try an invalid policy
	sp.EnforcementLevel = SentinelEnforcementLevelAdvisory
	sp.Policy = "blah 123"
	assert.NotNil(t, sp.Validate())
}

func TestSentinelPolicy_CacheKey(t *testing.T) {
	sp := &SentinelPolicy{
		Name:        "test",
		ModifyIndex: 10,
	}
	assert.Equal(t, "test:10", sp.CacheKey())
}

func TestSentinelPolicy_Compile(t *testing.T) {
	sp := &SentinelPolicy{
		Name:             "test",
		Description:      "Great policy",
		Scope:            SentinelScopeSubmitJob,
		EnforcementLevel: SentinelEnforcementLevelAdvisory,
		Policy:           "main = rule { true }",
	}

	f, fset, err := sp.Compile()
	assert.Nil(t, err)
	assert.NotNil(t, fset)
	assert.NotNil(t, f)
}

func TestQuotaSpec_Validate(t *testing.T) {
	cases := []struct {
		Name   string
		Spec   *QuotaSpec
		Errors []string
	}{
		{
			Name: "valid",
			Spec: &QuotaSpec{
				Name:        "foo",
				Description: "limit foo",
				Limits: []*QuotaLimit{
					{
						Region: "global",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
					},
				},
			},
		},
		{
			Name: "bad name, description, missing quota",
			Spec: &QuotaSpec{
				Name:        "*",
				Description: strings.Repeat("a", 1000),
			},
			Errors: []string{
				"invalid name",
				"description longer",
				"must provide at least one quota limit",
			},
		},
		{
			Name: "bad limit",
			Spec: &QuotaSpec{
				Limits: []*QuotaLimit{
					{},
				},
			},
			Errors: []string{
				"must provide a region",
				"must provide a region limit",
			},
		},
		{
			Name: "bad limit resources",
			Spec: &QuotaSpec{
				Limits: []*QuotaLimit{
					{
						Region: "foo",
						RegionLimit: &Resources{
							DiskMB: 500,
							Networks: []*NetworkResource{
								{},
							},
						},
					},
				},
			},
			Errors: []string{
				"limit disk",
				"limit networks",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			err := c.Spec.Validate()
			if err == nil {
				if len(c.Errors) != 0 {
					t.Fatalf("expected errors: %v", c.Errors)
				}
			} else {
				if len(c.Errors) == 0 {
					t.Fatalf("unexpected error: %v", err)
				} else {
					for _, exp := range c.Errors {
						if !strings.Contains(err.Error(), exp) {
							t.Fatalf("expected error to contain %q; got %v", exp, err)
						}
					}
				}
			}
		})
	}
}

func TestQuotaSpec_SetHash(t *testing.T) {
	assert := assert.New(t)
	qs := &QuotaSpec{
		Name:        "test",
		Description: "test limits",
		Limits: []*QuotaLimit{
			{
				Region: "foo",
				RegionLimit: &Resources{
					CPU: 5000,
				},
			},
		},
	}

	out1 := qs.SetHash()
	assert.NotNil(out1)
	assert.NotNil(qs.Hash)
	assert.Equal(out1, qs.Hash)

	qs.Name = "foo"
	out2 := qs.SetHash()
	assert.NotNil(out2)
	assert.NotNil(qs.Hash)
	assert.Equal(out2, qs.Hash)
	assert.NotEqual(out1, out2)
}

// Test that changing a region limit will also stimulate a hash change
func TestQuotaSpec_SetHash2(t *testing.T) {
	assert := assert.New(t)
	qs := &QuotaSpec{
		Name:        "test",
		Description: "test limits",
		Limits: []*QuotaLimit{
			{
				Region: "foo",
				RegionLimit: &Resources{
					CPU: 5000,
				},
			},
		},
	}

	out1 := qs.SetHash()
	assert.NotNil(out1)
	assert.NotNil(qs.Hash)
	assert.Equal(out1, qs.Hash)

	qs.Limits[0].RegionLimit.CPU = 2000
	out2 := qs.SetHash()
	assert.NotNil(out2)
	assert.NotNil(qs.Hash)
	assert.Equal(out2, qs.Hash)
	assert.NotEqual(out1, out2)
}

func TestQuotaUsage_Diff(t *testing.T) {
	cases := []struct {
		Name   string
		Usage  *QuotaUsage
		Spec   *QuotaSpec
		Create []string
		Delete []string
	}{
		{
			Name:   "noop",
			Create: []string{},
			Delete: []string{},
		},
		{
			Name: "no usage",
			Spec: &QuotaSpec{
				Name:        "foo",
				Description: "limit foo",
				Limits: []*QuotaLimit{
					{
						Region: "global",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x1},
					},
					{
						Region: "foo",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x2},
					},
				},
			},
			Create: []string{string(0x1), string(0x2)},
			Delete: []string{},
		},
		{
			Name: "no spec",
			Usage: &QuotaUsage{
				Name: "foo",
				Used: map[string]*QuotaLimit{
					"\x01": {
						Region: "global",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x1},
					},
					"\x02": {
						Region: "foo",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x2},
					},
				},
			},
			Create: []string{},
			Delete: []string{string(0x1), string(0x2)},
		},
		{
			Name: "both",
			Spec: &QuotaSpec{
				Name:        "foo",
				Description: "limit foo",
				Limits: []*QuotaLimit{
					{
						Region: "global",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x1},
					},
					{
						Region: "foo",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x2},
					},
				},
			},
			Usage: &QuotaUsage{
				Name: "foo",
				Used: map[string]*QuotaLimit{
					"\x01": {
						Region: "global",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x1},
					},
					"\x03": {
						Region: "bar",
						RegionLimit: &Resources{
							CPU:      5000,
							MemoryMB: 2000,
						},
						Hash: []byte{0x3},
					},
				},
			},
			Create: []string{string(0x2)},
			Delete: []string{string(0x3)},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actCreate, actDelete := c.Usage.DiffLimits(c.Spec)
			actCreateHashes := make([]string, 0, len(actCreate))
			actDeleteHashes := make([]string, 0, len(actDelete))
			for _, c := range actCreate {
				actCreateHashes = append(actCreateHashes, string(c.Hash))
			}
			for _, d := range actDelete {
				actDeleteHashes = append(actDeleteHashes, string(d.Hash))
			}

			sort.Strings(actCreateHashes)
			sort.Strings(actDeleteHashes)
			sort.Strings(c.Create)
			sort.Strings(c.Delete)
			assert.Equal(t, actCreateHashes, c.Create)
			assert.Equal(t, actDeleteHashes, c.Delete)
		})
	}
}

func TestQuotaLimit_Superset(t *testing.T) {
	l1 := &QuotaLimit{
		Region: "foo",
		RegionLimit: &Resources{
			CPU:      1000,
			MemoryMB: 1000,
		},
	}
	l2 := l1.Copy()
	l3 := l1.Copy()
	l3.RegionLimit.CPU++
	l3.RegionLimit.MemoryMB++

	superset, _ := l1.Superset(l2)
	assert.True(t, superset)

	superset, dimensions := l1.Superset(l3)
	assert.False(t, superset)
	assert.Len(t, dimensions, 2)

	l4 := l1.Copy()
	l4.RegionLimit.MemoryMB = 0
	l4.RegionLimit.CPU = 0
	superset, _ = l4.Superset(l3)
	assert.True(t, superset)

	l5 := l1.Copy()
	l5.RegionLimit.MemoryMB = -1
	l5.RegionLimit.CPU = -1
	superset, dimensions = l5.Superset(l3)
	assert.False(t, superset)
	assert.Len(t, dimensions, 2)
}
