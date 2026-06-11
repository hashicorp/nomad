// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"
	"time"

	"github.com/hashicorp/consul-template/config"
	"github.com/hashicorp/nomad/v2/ci"
	"github.com/shoenig/test/must"
)

func TestConfigRead(t *testing.T) {
	ci.Parallel(t)

	config := Config{}

	actual := config.Read("cake")
	must.Eq(t, "", actual)

	expected := "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.Read("cake")
	must.Eq(t, expected, actual)
}

func TestConfigReadDefault(t *testing.T) {
	ci.Parallel(t)

	config := Config{}

	expected := "vanilla"
	actual := config.ReadDefault("cake", expected)
	must.Eq(t, expected, actual)

	expected = "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.ReadDefault("cake", "vanilla")
	must.Eq(t, expected, actual)
}

func mockWaitConfig() *WaitConfig {
	return &WaitConfig{
		Min: new(5 * time.Second),
		Max: new(10 * time.Second),
	}
}

func TestWaitConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Wait     *WaitConfig
		Expected *WaitConfig
	}{
		{
			"fully-populated",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(10 * time.Second),
			},
		},
		{
			"min-only",
			&WaitConfig{
				Min: new(5 * time.Second),
			},
			&WaitConfig{
				Min: new(5 * time.Second),
			},
		},
		{
			"max-only",
			&WaitConfig{
				Max: new(5 * time.Second),
			},
			&WaitConfig{
				Max: new(5 * time.Second),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Equal(t, tc.Expected, tc.Wait.Copy())
		})
	}
}

func TestWaitConfig_IsEmpty(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Wait     *WaitConfig
		Expected bool
	}{
		{
			"is-nil",
			nil,
			true,
		},
		{
			"is-empty",
			&WaitConfig{},
			true,
		},
		{
			"is-not-empty",
			&WaitConfig{
				Min: new(10 * time.Second),
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Eq(t, tc.Expected, tc.Wait.IsEmpty())
		})
	}
}

func TestWaitConfig_IsEqual(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Wait     *WaitConfig
		Other    *WaitConfig
		Expected bool
	}{
		{
			"are-equal",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(10 * time.Second),
			},
			true,
		},
		{
			"min-different",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(4 * time.Second),
				Max: new(10 * time.Second),
			},
			false,
		},
		{
			"max-different",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(9 * time.Second),
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Eq(t, tc.Expected, tc.Wait.Equal(tc.Other))
		})
	}
}

func TestWaitConfig_IsValid(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Retry    *WaitConfig
		Expected string
	}{
		{
			"is-valid",
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(10 * time.Second),
			},
			"",
		},
		{
			"is-nil",
			nil,
			"is nil",
		},
		{
			"is-empty",
			&WaitConfig{},
			"or empty",
		},
		{
			"min-greater-than-max",
			&WaitConfig{
				Min: new(10 * time.Second),
				Max: new(5 * time.Second),
			},
			"greater than",
		},
		{
			"max-not-set",
			&WaitConfig{
				Min: new(10 * time.Second),
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Retry.Validate()
			if tc.Expected == "" {
				must.NoError(t, err)
			} else {
				must.ErrorContains(t, err, tc.Expected)
			}
		})
	}
}

func TestWaitConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Target   *WaitConfig
		Other    *WaitConfig
		Expected *WaitConfig
	}{
		{
			"all-fields",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(4 * time.Second),
				Max: new(9 * time.Second),
			},
			&WaitConfig{
				Min: new(4 * time.Second),
				Max: new(9 * time.Second),
			},
		},
		{
			"min-only",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(4 * time.Second),
				Max: new(10 * time.Second),
			},
			&WaitConfig{
				Min: new(4 * time.Second),
				Max: new(10 * time.Second),
			},
		},
		{
			"max-only",
			mockWaitConfig(),
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(9 * time.Second),
			},
			&WaitConfig{
				Min: new(5 * time.Second),
				Max: new(9 * time.Second),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			merged := tc.Target.Merge(tc.Other)
			must.Equal(t, tc.Expected, merged)
		})
	}
}

func TestWaitConfig_ToConsulTemplate(t *testing.T) {
	ci.Parallel(t)

	expected := config.WaitConfig{
		Enabled: new(true),
		Min:     new(5 * time.Second),
		Max:     new(10 * time.Second),
	}

	clientWaitConfig := &WaitConfig{
		Min: new(5 * time.Second),
		Max: new(10 * time.Second),
	}

	actual, err := clientWaitConfig.ToConsulTemplate()
	must.NoError(t, err)
	must.Eq(t, *expected.Enabled, *actual.Enabled)
	must.Eq(t, *expected.Min, *actual.Min)
	must.Eq(t, *expected.Max, *actual.Max)

	expected = config.WaitConfig{
		Enabled: new(false),
		Min:     new(0 * time.Second),
		Max:     new(0 * time.Second),
	}

	clientWaitConfig = &WaitConfig{
		Min: new(0 * time.Second),
		Max: new(0 * time.Second),
	}

	actual, err = clientWaitConfig.ToConsulTemplate()
	must.NoError(t, err)
	must.Eq(t, *expected.Min, *actual.Min)
	must.Eq(t, *expected.Max, *actual.Max)
}

func mockRetryConfig() *RetryConfig {
	return &RetryConfig{
		Attempts:      new(5),
		Backoff:       new(5 * time.Second),
		BackoffHCL:    "5s",
		MaxBackoff:    new(10 * time.Second),
		MaxBackoffHCL: "10s",
	}
}
func TestRetryConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Expected *RetryConfig
	}{
		{
			"fully-populated",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
		},
		{
			"attempts-only",
			&RetryConfig{
				Attempts: new(5),
			},
			&RetryConfig{
				Attempts: new(5),
			},
		},
		{
			"backoff-only",
			&RetryConfig{
				Backoff: new(5 * time.Second),
			},
			&RetryConfig{
				Backoff: new(5 * time.Second),
			},
		},
		{
			"backoff-hcl-only",
			&RetryConfig{
				BackoffHCL: "5s",
			},
			&RetryConfig{
				BackoffHCL: "5s",
			},
		},
		{
			"max-backoff-only",
			&RetryConfig{
				MaxBackoff: new(10 * time.Second),
			},
			&RetryConfig{
				MaxBackoff: new(10 * time.Second),
			},
		},
		{
			"max-backoff-hcl-only",
			&RetryConfig{
				MaxBackoffHCL: "10s",
			},
			&RetryConfig{
				MaxBackoffHCL: "10s",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Equal(t, tc.Expected, tc.Retry.Copy())
		})
	}
}

func TestRetryConfig_IsEmpty(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Expected bool
	}{
		{
			"is-nil",
			nil,
			true,
		},
		{
			"is-empty",
			&RetryConfig{},
			true,
		},
		{
			"is-not-empty",
			&RetryConfig{
				Attempts: new(12),
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Eq(t, tc.Expected, tc.Retry.IsEmpty())
		})
	}
}

func TestRetryConfig_IsEqual(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Other    *RetryConfig
		Expected bool
	}{
		{
			"are-equal",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			true,
		},
		{
			"attempts-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(4),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"backoff-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(4 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"backoff-hcl-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"max-backoff-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(9 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"max-backoff-hcl-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "9s",
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			must.Eq(t, tc.Expected, tc.Retry.Equal(tc.Other))
		})
	}
}

func TestRetryConfig_IsValid(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Expected string
	}{
		{
			"is-valid",
			&RetryConfig{
				Backoff:    new(5 * time.Second),
				MaxBackoff: new(10 * time.Second),
			},
			"",
		},
		{
			"is-nil",
			nil,
			"is nil",
		},
		{
			"is-empty",
			&RetryConfig{},
			"or empty",
		},
		{
			"backoff-greater-than-max-backoff",
			&RetryConfig{
				Backoff:    new(10 * time.Second),
				MaxBackoff: new(5 * time.Second),
			},
			"greater than max_backoff",
		},
		{
			"backoff-not-set",
			&RetryConfig{
				MaxBackoff: new(10 * time.Second),
			},
			"",
		},
		{
			"max-backoff-not-set",
			&RetryConfig{
				Backoff: new(2 * time.Minute),
			},
			"greater than default",
		},
		{
			"max-backoff-unbounded",
			&RetryConfig{
				Backoff:    new(10 * time.Second),
				MaxBackoff: new(0 * time.Second),
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Retry.Validate()
			if tc.Expected == "" {
				must.NoError(t, err)
			} else {
				must.ErrorContains(t, err, tc.Expected)
			}
		})
	}
}

func TestRetryConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name     string
		Target   *RetryConfig
		Other    *RetryConfig
		Expected *RetryConfig
	}{
		{
			"all-fields",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(4),
				Backoff:       new(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    new(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
			&RetryConfig{
				Attempts:      new(4),
				Backoff:       new(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    new(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
		},
		{
			"attempts-only",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(4),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			&RetryConfig{
				Attempts:      new(4),
				Backoff:       new(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    new(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
		},
		{
			"multi-field",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    new(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
			&RetryConfig{
				Attempts:      new(5),
				Backoff:       new(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    new(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			merged := tc.Target.Merge(tc.Other)
			must.Equal(t, tc.Expected, merged)
		})
	}
}

func TestRetryConfig_ToConsulTemplate(t *testing.T) {
	ci.Parallel(t)

	expected := config.RetryConfig{
		Enabled:    new(true),
		Attempts:   new(5),
		Backoff:    new(5 * time.Second),
		MaxBackoff: new(10 * time.Second),
	}

	actual := mockRetryConfig()
	must.Eq(t, *expected.Attempts, *actual.Attempts)
	must.Eq(t, *expected.Backoff, *actual.Backoff)
	must.Eq(t, *expected.MaxBackoff, *actual.MaxBackoff)
}
