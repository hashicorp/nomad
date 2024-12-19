// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func FuzzTaskScheduleCron(f *testing.F) {
	// valid values to compare against varying "now"s
	sched := TaskScheduleCron{
		Start:    "0 11 * * * *",
		End:      "0 13",
		Timezone: "UTC", // this timezone must match the fake "now" in Fuzz()
	}

	// seed the corpus with some target "now"s to time-travel to
	// args: year, month, day, hour, minute
	f.Add(0, 0, 0, 0, 0)        // zero
	f.Add(1970, 1, 0, 0, 0)     // epoch start
	f.Add(2000, 1, 0, 0, 0)     // y2k
	f.Add(2024, 12, 31, 23, 59) // end of this year
	f.Add(2038, 1, 19, 3, 0)    // y2kv2

	// regression tests:
	// schedule should be valid on the last day of the month.
	f.Add(2024, 6, 30, 12, 0)

	f.Fuzz(func(t *testing.T, year, month, day, hour, minute int) {
		now := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
		_, _, err := sched.Next(now)
		must.NoError(t, err,
			must.Sprintf("from=%q start=%q end=%q", now, sched.Start, sched.End))
	})
}

func TestTaskScheduleCron(t *testing.T) {
	ci.Parallel(t)

	// lil now helper
	getFrom := func(t *testing.T, val string) time.Time {
		t.Helper()
		from, err := time.Parse(time.RFC3339, val)
		must.NoError(t, err)
		return from
	}

	// july 1st 2024 at 1am is the eternal now.
	// it is a Monday.
	eternalNow := "2024-07-01T01:00:00Z"

	cases := []struct {
		name string

		now, start, end, tz string

		expectNext time.Duration
		expectEnd  time.Duration
		err        string
	}{
		{
			name:       "start < now < end",
			now:        "2024-07-01T01:30:00Z", // 01:30
			start:      "0 0 1 * * * *",        // 01:00
			end:        "0 0 2",                // 02:00
			expectNext: 0,                      // should be running
			expectEnd:  30 * time.Minute,
		},
		{
			name:       "now < start < end",
			now:        "2024-07-01T01:00:00Z", // 01:00
			start:      "0 10 1 * * * *",       // 01:10
			end:        "0 30 1",               // 01:30
			expectNext: 10 * time.Minute,       // run in 10 min
			expectEnd:  30 * time.Minute,
		},
		{
			name:       "start < end < now",
			now:        "2024-07-01T02:00:00Z", // 02:00
			start:      "0 0 1 * * * *",        // 01:00
			end:        "0 30 1",               // 01:30
			expectNext: 23 * time.Hour,         // run tomorrow @ 1am
			expectEnd:  23*time.Hour + 30*time.Minute,
		},
		{
			name:  "now < start",
			now:   "2024-07-01T01:00:00Z", // 01:00
			start: "0 10 1 * * * *",       // 01:10
			end:   "0 30 0",               // 00:30
			err:   "end cannot be sooner than start",
		},
		// real-life examples
		{
			name:       "before market monday",
			now:        "2024-07-01T08:00:00-04:00", // 08:00
			start:      "0 30 9 * * MON-FRI *",      // 09:30
			end:        "0 0 16",                    // 16:00
			tz:         "America/New_York",
			expectNext: time.Hour + 30*time.Minute,
			expectEnd:  8 * time.Hour,
		},
		{
			name:       "during market monday",
			now:        "2024-07-01T10:00:00-04:00", // 10:00
			start:      "0 30 9 * * MON-FRI *",      // 09:30
			end:        "0 0 16",                    // 16:00
			tz:         "America/New_York",
			expectNext: 0, // running now!
			expectEnd:  6 * time.Hour,
		},
		{
			name:       "after market monday",
			now:        "2024-07-01T23:00:00-04:00", // 23:00
			start:      "0 30 9 * * MON-FRI *",      // 09:30
			end:        "0 0 16",                    // 16:00
			tz:         "America/New_York",
			expectNext: 10*time.Hour + 30*time.Minute, // tuesday morning
			expectEnd:  17 * time.Hour,                // tuesday afternoon
		},
		{
			name:       "during market saturday",    // saturday!
			now:        "2024-07-06T10:00:00-04:00", // 10:00
			start:      "0 30 9 * * MON-FRI *",      // 09:30
			end:        "0 0 16",                    // 16:00
			tz:         "America/New_York",
			expectNext: (24+23)*time.Hour + 30*time.Minute, // monday morning
			expectEnd:  (24 + 24 + 6) * time.Hour,          // monday afternoon
		},
		{ // TODO: this one only works because friday farther back than 24h
			name:       "during market sunday",      // sunday!
			now:        "2024-07-07T10:00:00-04:00", // 10:00
			start:      "0 30 9 * * MON-FRI *",      // 09:30
			end:        "0 0 16",                    // 16:00
			tz:         "America/New_York",
			expectNext: 23*time.Hour + 30*time.Minute, // monday morning
			expectEnd:  (24 + 6) * time.Hour,          // monday afternoon
		},
		{
			name:       "end of the month",     // regression test for false "end cannot be sooner than start"
			now:        "2024-06-30T10:00:00Z", // june has 30 days
			start:      "0 9 * * * *",          // start before now
			end:        "0 11",                 // end after now
			expectNext: 0,                      // should be running
			expectEnd:  time.Hour,
		},
		// errors
		{
			name:  "bad tz",
			start: "any",
			end:   "any",
			tz:    "the moon",
			err:   "invalid timezone in schedule: unknown time zone the moon",
		},
		{
			name:  "no start",
			start: "",
			err:   `invalid start time in schedule: ""; missing field(s)`,
		},
		{
			name:  "bad start",
			start: "x",
			end:   "any",
			err:   `invalid start time in schedule: "x"; missing field(s)`,
		},
		{
			name:  "no end",
			start: "0 0 0 * * * *",
			end:   "",
			err:   `invalid end time in schedule: ""; missing field(s)`,
		},
		{
			name:  "bad end",
			start: "0 0 0 * * * *",
			end:   "ohno",
			err:   `invalid end time in schedule: "ohno"; syntax error in minute field: 'ohno'`,
		},
		{
			name:  "bad end minute",
			start: "0 0 0 * * * *",
			end:   "s m h",
			err:   `invalid end time in schedule: "s m h"; syntax error in second field: 's'`,
		},
		{
			name:  "bad end hour",
			start: "0 0 0 * * * *",
			end:   "0 1 h",
			err:   `invalid end time in schedule: "0 1 h"; syntax error in hour field: 'h'`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// default empty test case vals
			if tc.tz == "" {
				tc.tz = "UTC"
			}
			if tc.now == "" {
				tc.now = eternalNow
			}

			from := getFrom(t, tc.now)

			sched := TaskScheduleCron{
				Start:    tc.start,
				End:      tc.end,
				Timezone: tc.tz,
			}
			next, end, err := sched.Next(from)

			if tc.err == "" {
				must.NoError(t, err)
			} else {
				must.ErrorContains(t, err, tc.err)
			}
			test.Eq(t, tc.expectNext, next, test.Sprint("wrong next"))
			test.Eq(t, tc.expectEnd, end, test.Sprint("wrong end"))
		})
	}
}

func TestTaskSchedule_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		sched TaskSchedule
		err   string
	}{
		{
			name: "seconds",
			sched: TaskSchedule{
				Cron: &TaskScheduleCron{
					Start:    "0 00 09 * * * *",
					End:      "0 30 16",
					Timezone: "America/New_York",
				},
			},
			err: "cron.start must contain 6 fields",
		},
		{
			name: "slash",
			sched: TaskSchedule{
				Cron: &TaskScheduleCron{
					Start:    "5-59/5 09 * * * *",
					End:      "2 16",
					Timezone: "America/New_York",
				},
			},
			err: "cron.start must not contain",
		},
		{
			name: "leading zero",
			sched: TaskSchedule{
				Cron: &TaskScheduleCron{
					Start:    "00 09 * * * *",
					End:      "30 16",
					Timezone: "America/New_York",
				},
			},
		},
		{
			name: "no leading zero",
			sched: TaskSchedule{
				Cron: &TaskScheduleCron{
					Start:    "5 1 * * * *",
					End:      "0 2",
					Timezone: "America/New_York",
				},
			},
		},
		{
			name: "eastern",
			sched: TaskSchedule{
				Cron: &TaskScheduleCron{
					Start:    "0 9 * * * *",
					End:      "30 16",
					Timezone: "EST5EDT",
				},
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sched.Validate()
			if tc.err == "" {
				must.NoError(t, err)
			} else {
				must.ErrorContains(t, err, tc.err)
			}
		})
	}
}
