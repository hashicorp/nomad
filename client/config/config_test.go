package config

import (
	"testing"
	"time"

	"github.com/hashicorp/consul-template/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

func TestConfigRead(t *testing.T) {
	config := Config{}

	actual := config.Read("cake")
	if actual != "" {
		t.Errorf("Expected empty string; found %s", actual)
	}

	expected := "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.Read("cake")
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}
}

func TestConfigReadDefault(t *testing.T) {
	config := Config{}

	expected := "vanilla"
	actual := config.ReadDefault("cake", expected)
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}

	expected = "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.ReadDefault("cake", "vanilla")
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}
}

func mockWaitConfig() *WaitConfig {
	return &WaitConfig{
		Min: helper.TimeToPtr(5 * time.Second),
		Max: helper.TimeToPtr(10 * time.Second),
	}
}

func TestWaitConfig_Copy(t *testing.T) {
	cases := []struct {
		Name     string
		Wait     *WaitConfig
		Expected *WaitConfig
	}{
		{
			"fully-populated",
			mockWaitConfig(),
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
			},
		},
		{
			"min-only",
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
			},
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
			},
		},
		{
			"max-only",
			&WaitConfig{
				Max: helper.TimeToPtr(5 * time.Second),
			},
			&WaitConfig{
				Max: helper.TimeToPtr(5 * time.Second),
			},
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			result := _case.Expected.Equals(_case.Wait.Copy())
			if !result {
				t.Logf("\nExpected %v\n   Found %v", _case.Expected, result)
			}
			require.True(t, result)
		})
	}
}

func TestWaitConfig_IsEmpty(t *testing.T) {
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
				Min: helper.TimeToPtr(10 * time.Second),
			},
			false,
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			require.Equal(t, _case.Expected, _case.Wait.IsEmpty())
		})
	}
}

func TestWaitConfig_IsEqual(t *testing.T) {
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
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
			},
			true,
		},
		{
			"min-different",
			mockWaitConfig(),
			&WaitConfig{
				Min: helper.TimeToPtr(4 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
			},
			false,
		},
		{
			"max-different",
			mockWaitConfig(),
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(9 * time.Second),
			},
			false,
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			require.Equal(t, _case.Expected, _case.Wait.Equals(_case.Other))
		})
	}
}

func TestWaitConfig_IsValid(t *testing.T) {
	cases := []struct {
		Name     string
		Retry    *WaitConfig
		Expected string
	}{
		{
			"is-valid",
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
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
				Min: helper.TimeToPtr(10 * time.Second),
				Max: helper.TimeToPtr(5 * time.Second),
			},
			"greater than",
		},
		{
			"max-not-set",
			&WaitConfig{
				Min: helper.TimeToPtr(10 * time.Second),
			},
			"",
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			if _case.Expected == "" {
				require.Nil(t, _case.Retry.Validate())
			} else {
				err := _case.Retry.Validate()
				require.Contains(t, err.Error(), _case.Expected)
			}
		})
	}
}

func TestWaitConfig_Merge(t *testing.T) {
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
				Min: helper.TimeToPtr(4 * time.Second),
				Max: helper.TimeToPtr(9 * time.Second),
			},
			&WaitConfig{
				Min: helper.TimeToPtr(4 * time.Second),
				Max: helper.TimeToPtr(9 * time.Second),
			},
		},
		{
			"min-only",
			mockWaitConfig(),
			&WaitConfig{
				Min: helper.TimeToPtr(4 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
			},
			&WaitConfig{
				Min: helper.TimeToPtr(4 * time.Second),
				Max: helper.TimeToPtr(10 * time.Second),
			},
		},
		{
			"max-only",
			mockWaitConfig(),
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(9 * time.Second),
			},
			&WaitConfig{
				Min: helper.TimeToPtr(5 * time.Second),
				Max: helper.TimeToPtr(9 * time.Second),
			},
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			merged := _case.Target.Merge(_case.Other)
			result := _case.Expected.Equals(merged)
			if !result {
				t.Logf("\nExpected %v\n   Found %v", _case.Expected, merged)
			}
			require.True(t, result)
		})
	}
}

func TestWaitConfig_ToConsulTemplate(t *testing.T) {
	expected := config.WaitConfig{
		Enabled: helper.BoolToPtr(true),
		Min:     helper.TimeToPtr(5 * time.Second),
		Max:     helper.TimeToPtr(10 * time.Second),
	}

	clientWaitConfig := &WaitConfig{
		Min: helper.TimeToPtr(5 * time.Second),
		Max: helper.TimeToPtr(10 * time.Second),
	}

	actual, err := clientWaitConfig.ToConsulTemplate()
	require.NoError(t, err)
	require.Equal(t, *expected.Min, *actual.Min)
	require.Equal(t, *expected.Max, *actual.Max)
}

func mockRetryConfig() *RetryConfig {
	return &RetryConfig{
		Attempts:      helper.IntToPtr(5),
		Backoff:       helper.TimeToPtr(5 * time.Second),
		BackoffHCL:    "5s",
		MaxBackoff:    helper.TimeToPtr(10 * time.Second),
		MaxBackoffHCL: "10s",
	}
}
func TestRetryConfig_Copy(t *testing.T) {
	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Expected *RetryConfig
	}{
		{
			"fully-populated",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
		},
		{
			"attempts-only",
			&RetryConfig{
				Attempts: helper.IntToPtr(5),
			},
			&RetryConfig{
				Attempts: helper.IntToPtr(5),
			},
		},
		{
			"backoff-only",
			&RetryConfig{
				Backoff: helper.TimeToPtr(5 * time.Second),
			},
			&RetryConfig{
				Backoff: helper.TimeToPtr(5 * time.Second),
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
				MaxBackoff: helper.TimeToPtr(10 * time.Second),
			},
			&RetryConfig{
				MaxBackoff: helper.TimeToPtr(10 * time.Second),
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

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			result := _case.Expected.Equals(_case.Retry.Copy())
			if !result {
				t.Logf("\nExpected %v\n   Found %v", _case.Expected, result)
			}
			require.True(t, result)
		})
	}
}

func TestRetryConfig_IsEmpty(t *testing.T) {
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
				Attempts: helper.IntToPtr(12),
			},
			false,
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			require.Equal(t, _case.Expected, _case.Retry.IsEmpty())
		})
	}
}

func TestRetryConfig_IsEqual(t *testing.T) {
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
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			true,
		},
		{
			"attempts-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(4),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"backoff-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(4 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"backoff-hcl-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"max-backoff-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(9 * time.Second),
				MaxBackoffHCL: "10s",
			},
			false,
		},
		{
			"max-backoff-hcl-different",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "9s",
			},
			false,
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			require.Equal(t, _case.Expected, _case.Retry.Equals(_case.Other))
		})
	}
}

func TestRetryConfig_IsValid(t *testing.T) {
	cases := []struct {
		Name     string
		Retry    *RetryConfig
		Expected string
	}{
		{
			"is-valid",
			&RetryConfig{
				Backoff:    helper.TimeToPtr(5 * time.Second),
				MaxBackoff: helper.TimeToPtr(10 * time.Second),
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
				Backoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoff: helper.TimeToPtr(5 * time.Second),
			},
			"greater than max_backoff",
		},
		{
			"backoff-not-set",
			&RetryConfig{
				MaxBackoff: helper.TimeToPtr(10 * time.Second),
			},
			"",
		},
		{
			"max-backoff-not-set",
			&RetryConfig{
				Backoff: helper.TimeToPtr(2 * time.Minute),
			},
			"greater than default",
		},
		{
			"max-backoff-unbounded",
			&RetryConfig{
				Backoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoff: helper.TimeToPtr(0 * time.Second),
			},
			"",
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			if _case.Expected == "" {
				require.Nil(t, _case.Retry.Validate())
			} else {
				err := _case.Retry.Validate()
				require.Contains(t, err.Error(), _case.Expected)
			}
		})
	}
}

func TestRetryConfig_Merge(t *testing.T) {
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
				Attempts:      helper.IntToPtr(4),
				Backoff:       helper.TimeToPtr(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    helper.TimeToPtr(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
			&RetryConfig{
				Attempts:      helper.IntToPtr(4),
				Backoff:       helper.TimeToPtr(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    helper.TimeToPtr(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
		},
		{
			"attempts-only",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(4),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
			&RetryConfig{
				Attempts:      helper.IntToPtr(4),
				Backoff:       helper.TimeToPtr(5 * time.Second),
				BackoffHCL:    "5s",
				MaxBackoff:    helper.TimeToPtr(10 * time.Second),
				MaxBackoffHCL: "10s",
			},
		},
		{
			"multi-field",
			mockRetryConfig(),
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    helper.TimeToPtr(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
			&RetryConfig{
				Attempts:      helper.IntToPtr(5),
				Backoff:       helper.TimeToPtr(4 * time.Second),
				BackoffHCL:    "4s",
				MaxBackoff:    helper.TimeToPtr(9 * time.Second),
				MaxBackoffHCL: "9s",
			},
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			merged := _case.Target.Merge(_case.Other)
			result := _case.Expected.Equals(merged)
			if !result {
				t.Logf("\nExpected %v\n   Found %v", _case.Expected, merged)
			}
			require.True(t, result)
		})
	}
}

func TestRetryConfig_ToConsulTemplate(t *testing.T) {
	expected := config.RetryConfig{
		Enabled:    helper.BoolToPtr(true),
		Attempts:   helper.IntToPtr(5),
		Backoff:    helper.TimeToPtr(5 * time.Second),
		MaxBackoff: helper.TimeToPtr(10 * time.Second),
	}

	actual := mockRetryConfig()

	require.Equal(t, *expected.Attempts, *actual.Attempts)
	require.Equal(t, *expected.Backoff, *actual.Backoff)
	require.Equal(t, *expected.MaxBackoff, *actual.MaxBackoff)
}
