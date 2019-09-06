package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
