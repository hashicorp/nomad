package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/stretchr/testify/require"
)

func TestArtifactConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	a := DefaultArtifactConfig()
	b := a.Copy()
	require.Equal(t, a, b)

	b.HTTPReadTimeout = pointer.Of("5m")
	b.HTTPMaxSize = pointer.Of("2MB")
	b.GitTimeout = pointer.Of("3m")
	b.HgTimeout = pointer.Of("2m")
	require.NotEqual(t, a, b)
}

func TestArtifactConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		source   *ArtifactConfig
		other    *ArtifactConfig
		expected *ArtifactConfig
	}{
		{
			name: "merge all fields",
			source: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			other: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("5m"),
				HTTPMaxSize:     pointer.Of("2GB"),
				GCSTimeout:      pointer.Of("1m"),
				GitTimeout:      pointer.Of("2m"),
				HgTimeout:       pointer.Of("3m"),
				S3Timeout:       pointer.Of("4m"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("5m"),
				HTTPMaxSize:     pointer.Of("2GB"),
				GCSTimeout:      pointer.Of("1m"),
				GitTimeout:      pointer.Of("2m"),
				HgTimeout:       pointer.Of("3m"),
				S3Timeout:       pointer.Of("4m"),
			},
		},
		{
			name:   "null source",
			source: nil,
			other: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("5m"),
				HTTPMaxSize:     pointer.Of("2GB"),
				GCSTimeout:      pointer.Of("1m"),
				GitTimeout:      pointer.Of("2m"),
				HgTimeout:       pointer.Of("3m"),
				S3Timeout:       pointer.Of("4m"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("5m"),
				HTTPMaxSize:     pointer.Of("2GB"),
				GCSTimeout:      pointer.Of("1m"),
				GitTimeout:      pointer.Of("2m"),
				HgTimeout:       pointer.Of("3m"),
				S3Timeout:       pointer.Of("4m"),
			},
		},
		{
			name: "null other",
			source: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			other: nil,
			expected: &ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.source.Merge(tc.other)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestArtifactConfig_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name          string
		config        func(*ArtifactConfig)
		expectedError string
	}{
		{
			name:          "default config is valid",
			config:        nil,
			expectedError: "",
		},
		{
			name: "missing http read timeout",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = nil
			},
			expectedError: "http_read_timeout must be set",
		},
		{
			name: "http read timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("invalid")
			},
			expectedError: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("")
			},
			expectedError: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "http read timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("-10m")
			},
			expectedError: "http_read_timeout must be > 0",
		},
		{
			name: "http max size is missing",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = nil
			},
			expectedError: "http_max_size must be set",
		},
		{
			name: "http max size is invalid",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("invalid")
			},
			expectedError: "http_max_size not a valid size",
		},
		{
			name: "http max size is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("")
			},
			expectedError: "http_max_size not a valid size",
		},
		{
			name: "http max size is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "http max size is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("-l0MB")
			},
			expectedError: "http_max_size not a valid size",
		},
		{
			name: "gcs timeout is missing",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = nil
			},
			expectedError: "gcs_timeout must be set",
		},
		{
			name: "gcs timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("invalid")
			},
			expectedError: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("")
			},
			expectedError: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "gcs timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("-l0m")
			},
			expectedError: "gcs_timeout not a valid duration",
		},
		{
			name: "git timeout is missing",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = nil
			},
			expectedError: "git_timeout must be set",
		},
		{
			name: "git timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("invalid")
			},
			expectedError: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("")
			},
			expectedError: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "git timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("-l0m")
			},
			expectedError: "git_timeout not a valid duration",
		},
		{
			name: "hg timeout is missing",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = nil
			},
			expectedError: "hg_timeout must be set",
		},
		{
			name: "hg timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("invalid")
			},
			expectedError: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("")
			},
			expectedError: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "hg timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("-l0m")
			},
			expectedError: "hg_timeout not a valid duration",
		},
		{
			name: "s3 timeout is missing",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = nil
			},
			expectedError: "s3_timeout must be set",
		},
		{
			name: "s3 timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("invalid")
			},
			expectedError: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is empty",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("")
			},
			expectedError: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is zero",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("0")
			},
			expectedError: "",
		},
		{
			name: "s3 timeout is negative",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("-l0m")
			},
			expectedError: "s3_timeout not a valid duration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := DefaultArtifactConfig()
			if tc.config != nil {
				tc.config(a)
			}

			err := a.Validate()
			if tc.expectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
