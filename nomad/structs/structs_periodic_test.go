// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriodicConfig_DSTChange_Transitions(t *testing.T) {
	ci.Parallel(t)

	locName := "America/Los_Angeles"
	loc, err := time.LoadLocation(locName)
	require.NoError(t, err)

	cases := []struct {
		name     string
		pattern  string
		initTime time.Time
		expected []time.Time
	}{
		{
			"normal time",
			"0 2 * * * 2019",
			time.Date(2019, time.February, 7, 1, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.February, 7, 2, 0, 0, 0, loc),
				time.Date(2019, time.February, 8, 2, 0, 0, 0, loc),
				time.Date(2019, time.February, 9, 2, 0, 0, 0, loc),
			},
		},
		{
			"Spring forward but not in switch time",
			"0 4 * * * 2019",
			time.Date(2019, time.March, 9, 1, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.March, 9, 4, 0, 0, 0, loc),
				time.Date(2019, time.March, 10, 4, 0, 0, 0, loc),
				time.Date(2019, time.March, 11, 4, 0, 0, 0, loc),
			},
		},
		{
			"Spring forward at a skipped time odd",
			"2 2 * * * 2019",
			time.Date(2019, time.March, 9, 1, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.March, 9, 2, 2, 0, 0, loc),
				// no time in March 10!
				time.Date(2019, time.March, 11, 2, 2, 0, 0, loc),
				time.Date(2019, time.March, 12, 2, 2, 0, 0, loc),
			},
		},
		{
			"Spring forward at a skipped time",
			"1 2 * * * 2019",
			time.Date(2019, time.March, 9, 1, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.March, 9, 2, 1, 0, 0, loc),
				// no time in March 8!
				time.Date(2019, time.March, 11, 2, 1, 0, 0, loc),
				time.Date(2019, time.March, 12, 2, 1, 0, 0, loc),
			},
		},
		{
			"Spring forward at a skipped time boundary",
			"0 2 * * * 2019",
			time.Date(2019, time.March, 9, 1, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.March, 9, 2, 0, 0, 0, loc),
				// no time in March 8!
				time.Date(2019, time.March, 11, 2, 0, 0, 0, loc),
				time.Date(2019, time.March, 12, 2, 0, 0, 0, loc),
			},
		},
		{
			"Spring forward at a boundary of repeating time",
			"0 1 * * * 2019",
			time.Date(2019, time.March, 9, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.March, 9, 1, 0, 0, 0, loc),
				time.Date(2019, time.March, 10, 0, 0, 0, 0, loc).Add(1 * time.Hour),
				time.Date(2019, time.March, 11, 1, 0, 0, 0, loc),
				time.Date(2019, time.March, 12, 1, 0, 0, 0, loc),
			},
		},
		{
			"Fall back: before transition",
			"30 0 * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc),
				time.Date(2019, time.November, 4, 0, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 0, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 0, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: after transition",
			"30 3 * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 4, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 3, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: after transition starting in repeated span before",
			"30 3 * * * 2019",
			time.Date(2019, time.November, 3, 0, 10, 0, 0, loc).Add(1 * time.Hour),
			[]time.Time{
				time.Date(2019, time.November, 3, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 4, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 3, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: after transition starting in repeated span after",
			"30 3 * * * 2019",
			time.Date(2019, time.November, 3, 0, 10, 0, 0, loc).Add(2 * time.Hour),
			[]time.Time{
				time.Date(2019, time.November, 3, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 4, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 3, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 3, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: in repeated region",
			"30 1 * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc).Add(1 * time.Hour),
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc).Add(2 * time.Hour),
				time.Date(2019, time.November, 4, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 1, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: in repeated region boundary",
			"0 1 * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 0, 0, 0, loc).Add(1 * time.Hour),
				time.Date(2019, time.November, 3, 0, 0, 0, 0, loc).Add(2 * time.Hour),
				time.Date(2019, time.November, 4, 1, 0, 0, 0, loc),
				time.Date(2019, time.November, 5, 1, 0, 0, 0, loc),
				time.Date(2019, time.November, 6, 1, 0, 0, 0, loc),
			},
		},
		{
			"Fall back: in repeated region boundary 2",
			"0 2 * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 0, 0, 0, loc).Add(3 * time.Hour),
				time.Date(2019, time.November, 4, 2, 0, 0, 0, loc),
				time.Date(2019, time.November, 5, 2, 0, 0, 0, loc),
				time.Date(2019, time.November, 6, 2, 0, 0, 0, loc),
			},
		},
		{
			"Fall back: in repeated region, starting from within region",
			"30 1 * * * 2019",
			time.Date(2019, time.November, 3, 0, 40, 0, 0, loc).Add(1 * time.Hour),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc).Add(2 * time.Hour),
				time.Date(2019, time.November, 4, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 1, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: in repeated region, starting from within region 2",
			"30 1 * * * 2019",
			time.Date(2019, time.November, 3, 0, 40, 0, 0, loc).Add(2 * time.Hour),
			[]time.Time{
				time.Date(2019, time.November, 4, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 5, 1, 30, 0, 0, loc),
				time.Date(2019, time.November, 6, 1, 30, 0, 0, loc),
			},
		},
		{
			"Fall back: wildcard",
			"30 * * * * 2019",
			time.Date(2019, time.November, 3, 0, 0, 0, 0, loc),
			[]time.Time{
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc),
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc).Add(1 * time.Hour),
				time.Date(2019, time.November, 3, 0, 30, 0, 0, loc).Add(2 * time.Hour),
				time.Date(2019, time.November, 3, 2, 30, 0, 0, loc),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &PeriodicConfig{
				Enabled:  true,
				SpecType: PeriodicSpecCron,
				Spec:     c.pattern,
				TimeZone: locName,
			}
			p.Canonicalize()

			starting := c.initTime
			for _, next := range c.expected {
				n, err := p.Next(starting)
				assert.NoError(t, err)
				assert.Equalf(t, next, n, "next time of %v", starting)

				starting = next
			}
		})
	}
}

func TestPeriodConfig_DSTSprintForward_Property(t *testing.T) {
	ci.Parallel(t)

	locName := "America/Los_Angeles"
	loc, err := time.LoadLocation(locName)
	require.NoError(t, err)

	cronExprs := []string{
		"* * * * *",
		"0 2 * * *",
		"* 1 * * *",
	}

	times := []time.Time{
		// spring forward
		time.Date(2019, time.March, 11, 0, 0, 0, 0, loc),
		time.Date(2019, time.March, 10, 0, 0, 0, 0, loc),
		time.Date(2019, time.March, 11, 0, 0, 0, 0, loc),

		// leap backwards
		time.Date(2019, time.November, 4, 0, 0, 0, 0, loc),
		time.Date(2019, time.November, 5, 0, 0, 0, 0, loc),
		time.Date(2019, time.November, 6, 0, 0, 0, 0, loc),
	}

	testSpan := 4 * time.Hour

	testCase := func(t *testing.T, cronExpr string, init time.Time) {
		p := &PeriodicConfig{
			Enabled:  true,
			SpecType: PeriodicSpecCron,
			Spec:     cronExpr,
			TimeZone: "America/Los_Angeles",
		}
		p.Canonicalize()

		lastNext := init
		for start := init; start.Before(init.Add(testSpan)); start = start.Add(1 * time.Minute) {
			next, err := p.Next(start)
			require.NoError(t, err)
			require.Truef(t, next.After(start),
				"next(%v) = %v is not after init time", start, next)

			if start.Before(lastNext) {
				require.Equalf(t, lastNext, next, "next(%v) = %v is earlier than previously known next %v",
					start, next, lastNext)
			}
			if strings.HasPrefix(cronExpr, "* * ") {
				require.Equalf(t, next.Sub(start), 1*time.Minute,
					"next(%v) = %v is the next minute", start, next)
			}

			lastNext = next
		}
	}

	for _, cron := range cronExprs {
		for _, startTime := range times {
			t.Run(fmt.Sprintf("%v: %v", cron, startTime), func(t *testing.T) {
				testCase(t, cron, startTime)
			})
		}
	}
}
