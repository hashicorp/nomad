// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestArtifactConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	a := DefaultArtifactConfig()
	b := a.Copy()
	must.Equal(t, a, b)
	must.Equal(t, b, a)

	b.HTTPReadTimeout = pointer.Of("5m")
	b.HTTPMaxSize = pointer.Of("2MB")
	b.GitTimeout = pointer.Of("3m")
	b.HgTimeout = pointer.Of("2m")
	b.DecompressionFileCountLimit = pointer.Of(7)
	b.DecompressionSizeLimit = pointer.Of("2GB")
	must.NotEqual(t, a, b)
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
				HTTPReadTimeout:             pointer.Of("30m"),
				HTTPMaxSize:                 pointer.Of("100GB"),
				GCSTimeout:                  pointer.Of("30m"),
				GitTimeout:                  pointer.Of("30m"),
				HgTimeout:                   pointer.Of("30m"),
				S3Timeout:                   pointer.Of("30m"),
				DecompressionFileCountLimit: pointer.Of(4096),
				DecompressionSizeLimit:      pointer.Of("100GB"),
				DisableFilesystemIsolation:  pointer.Of(false),
				SetEnvironmentVariables:     pointer.Of(""),
			},
			other: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("5m"),
				HTTPMaxSize:                 pointer.Of("2GB"),
				GCSTimeout:                  pointer.Of("1m"),
				GitTimeout:                  pointer.Of("2m"),
				HgTimeout:                   pointer.Of("3m"),
				S3Timeout:                   pointer.Of("4m"),
				DecompressionFileCountLimit: pointer.Of(100),
				DecompressionSizeLimit:      pointer.Of("8GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("5m"),
				HTTPMaxSize:                 pointer.Of("2GB"),
				GCSTimeout:                  pointer.Of("1m"),
				GitTimeout:                  pointer.Of("2m"),
				HgTimeout:                   pointer.Of("3m"),
				S3Timeout:                   pointer.Of("4m"),
				DecompressionFileCountLimit: pointer.Of(100),
				DecompressionSizeLimit:      pointer.Of("8GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
		},
		{
			name:   "null source",
			source: nil,
			other: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("5m"),
				HTTPMaxSize:                 pointer.Of("2GB"),
				GCSTimeout:                  pointer.Of("1m"),
				GitTimeout:                  pointer.Of("2m"),
				HgTimeout:                   pointer.Of("3m"),
				S3Timeout:                   pointer.Of("4m"),
				DecompressionFileCountLimit: pointer.Of(100),
				DecompressionSizeLimit:      pointer.Of("8GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("5m"),
				HTTPMaxSize:                 pointer.Of("2GB"),
				GCSTimeout:                  pointer.Of("1m"),
				GitTimeout:                  pointer.Of("2m"),
				HgTimeout:                   pointer.Of("3m"),
				S3Timeout:                   pointer.Of("4m"),
				DecompressionFileCountLimit: pointer.Of(100),
				DecompressionSizeLimit:      pointer.Of("8GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
		},
		{
			name: "null other",
			source: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("30m"),
				HTTPMaxSize:                 pointer.Of("100GB"),
				GCSTimeout:                  pointer.Of("30m"),
				GitTimeout:                  pointer.Of("30m"),
				HgTimeout:                   pointer.Of("30m"),
				S3Timeout:                   pointer.Of("30m"),
				DecompressionFileCountLimit: pointer.Of(4096),
				DecompressionSizeLimit:      pointer.Of("100GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
			other: nil,
			expected: &ArtifactConfig{
				HTTPReadTimeout:             pointer.Of("30m"),
				HTTPMaxSize:                 pointer.Of("100GB"),
				GCSTimeout:                  pointer.Of("30m"),
				GitTimeout:                  pointer.Of("30m"),
				HgTimeout:                   pointer.Of("30m"),
				S3Timeout:                   pointer.Of("30m"),
				DecompressionFileCountLimit: pointer.Of(4096),
				DecompressionSizeLimit:      pointer.Of("100GB"),
				DisableFilesystemIsolation:  pointer.Of(true),
				SetEnvironmentVariables:     pointer.Of("FOO,BAR"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.source.Merge(tc.other)
			must.Equal(t, tc.expected, got)
		})
	}
}

func TestArtifactConfig_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		config func(*ArtifactConfig)
		expErr string
	}{
		{
			name:   "default config is valid",
			config: nil,
			expErr: "",
		},
		{
			name: "missing http read timeout",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = nil
			},
			expErr: "http_read_timeout must be set",
		},
		{
			name: "http read timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("invalid")
			},
			expErr: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("")
			},
			expErr: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "http read timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = pointer.Of("-10m")
			},
			expErr: "http_read_timeout must be > 0",
		},
		{
			name: "http max size is missing",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = nil
			},
			expErr: "http_max_size must be set",
		},
		{
			name: "http max size is invalid",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("invalid")
			},
			expErr: "http_max_size not a valid size",
		},
		{
			name: "http max size is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("")
			},
			expErr: "http_max_size not a valid size",
		},
		{
			name: "http max size is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "http max size is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = pointer.Of("-l0MB")
			},
			expErr: "http_max_size not a valid size",
		},
		{
			name: "gcs timeout is missing",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = nil
			},
			expErr: "gcs_timeout must be set",
		},
		{
			name: "gcs timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("invalid")
			},
			expErr: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("")
			},
			expErr: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "gcs timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = pointer.Of("-l0m")
			},
			expErr: "gcs_timeout not a valid duration",
		},
		{
			name: "git timeout is missing",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = nil
			},
			expErr: "git_timeout must be set",
		},
		{
			name: "git timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("invalid")
			},
			expErr: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("")
			},
			expErr: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "git timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = pointer.Of("-l0m")
			},
			expErr: "git_timeout not a valid duration",
		},
		{
			name: "hg timeout is missing",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = nil
			},
			expErr: "hg_timeout must be set",
		},
		{
			name: "hg timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("invalid")
			},
			expErr: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("")
			},
			expErr: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "hg timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = pointer.Of("-l0m")
			},
			expErr: "hg_timeout not a valid duration",
		},
		{
			name: "s3 timeout is missing",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = nil
			},
			expErr: "s3_timeout must be set",
		},
		{
			name: "s3 timeout is invalid",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("invalid")
			},
			expErr: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is empty",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("")
			},
			expErr: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is zero",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("0")
			},
			expErr: "",
		},
		{
			name: "s3 timeout is negative",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = pointer.Of("-l0m")
			},
			expErr: "s3_timeout not a valid duration",
		},
		{
			name: "decompression file count limit is nil",
			config: func(a *ArtifactConfig) {
				a.DecompressionFileCountLimit = nil
			},
			expErr: "decompression_file_count_limit must not be nil",
		},
		{
			name: "decompression file count limit is negative",
			config: func(a *ArtifactConfig) {
				a.DecompressionFileCountLimit = pointer.Of(-1)
			},
			expErr: "decompression_file_count_limit must be >= 0 but found -1",
		},
		{
			name: "decompression size limit is nil",
			config: func(a *ArtifactConfig) {
				a.DecompressionSizeLimit = nil
			},
			expErr: "decompression_size_limit must not be nil",
		},
		{
			name: "decompression size limit is negative",
			config: func(a *ArtifactConfig) {
				a.DecompressionSizeLimit = pointer.Of("-1GB")
			},
			expErr: "decompression_size_limit is not a valid size",
		},
		{
			name: "fs isolation not set",
			config: func(a *ArtifactConfig) {
				a.DisableFilesystemIsolation = nil
			},
			expErr: "disable_filesystem_isolation must be set",
		},
		{
			name: "env not set",
			config: func(a *ArtifactConfig) {
				a.SetEnvironmentVariables = nil
			},
			expErr: "set_environment_variables must be set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := DefaultArtifactConfig()
			if tc.config != nil {
				tc.config(a)
			}

			err := a.Validate()
			if tc.expErr != "" {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.expErr)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
