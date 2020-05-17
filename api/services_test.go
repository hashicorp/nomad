package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_CheckRestart asserts Service.CheckRestart settings are properly
// inherited by Checks.
func TestService_CheckRestart(t *testing.T) {

	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		CheckRestart: &CheckRestart{
			Limit:          11,
			Grace:          timeToPtr(11 * time.Second),
			IgnoreWarnings: true,
		},
		Checks: []ServiceCheck{
			{
				Name: "all-set",
				CheckRestart: &CheckRestart{
					Limit:          22,
					Grace:          timeToPtr(22 * time.Second),
					IgnoreWarnings: true,
				},
			},
			{
				Name: "some-set",
				CheckRestart: &CheckRestart{
					Limit: 33,
					Grace: timeToPtr(33 * time.Second),
				},
			},
			{
				Name: "unset",
			},
		},
	}

	service.Canonicalize(task, tg, job)
	assert.Equal(t, service.Checks[0].CheckRestart.Limit, 22)
	assert.Equal(t, *service.Checks[0].CheckRestart.Grace, 22*time.Second)
	assert.True(t, service.Checks[0].CheckRestart.IgnoreWarnings)

	assert.Equal(t, service.Checks[1].CheckRestart.Limit, 33)
	assert.Equal(t, *service.Checks[1].CheckRestart.Grace, 33*time.Second)
	assert.True(t, service.Checks[1].CheckRestart.IgnoreWarnings)

	assert.Equal(t, service.Checks[2].CheckRestart.Limit, 11)
	assert.Equal(t, *service.Checks[2].CheckRestart.Grace, 11*time.Second)
	assert.True(t, service.Checks[2].CheckRestart.IgnoreWarnings)
}

// TestService_Connect asserts Service.Connect settings are properly
// inherited by Checks.
func TestService_Connect(t *testing.T) {

	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		Connect: &ConsulConnect{
			SidecarService: &ConsulSidecarService{
				Proxy: &ConsulProxy{
					Upstreams: []*ConsulUpstream{
						{
							DestinationName: "upstream",
							LocalBindPort:   80,
						},
					},
					LocalServicePort: 8000,
				},
			},
		},
	}

	service.Canonicalize(task, tg, job)
	proxy := service.Connect.SidecarService.Proxy
	assert.Equal(t, proxy.Upstreams[0].LocalBindPort, 80)
	assert.Equal(t, proxy.Upstreams[0].DestinationName, "upstream")
	assert.Equal(t, proxy.LocalServicePort, 8000)
}

func TestService_Tags(t *testing.T) {
	r := require.New(t)

	// canonicalize does not modify eto or tags
	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		Tags:              []string{"a", "b"},
		CanaryTags:        []string{"c", "d"},
		EnableTagOverride: true,
	}

	service.Canonicalize(task, tg, job)
	r.True(service.EnableTagOverride)
	r.Equal([]string{"a", "b"}, service.Tags)
	r.Equal([]string{"c", "d"}, service.CanaryTags)
}
