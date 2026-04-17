// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestArtifactConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	a := DefaultArtifactConfig()
	a.FilesystemIsolationExtraPaths = []string{
		"f:r:/dev/urandom",
		"d:rx:/opt/bin",
		"d:r:/tmp/stash",
	}
	b := a.Copy()
	must.Equal(t, a, b)
	must.Equal(t, b, a)

	b.HTTPReadTimeout = new("5m")
	b.HTTPMaxSize = new("2MB")
	b.GitTimeout = new("3m")
	b.HgTimeout = new("2m")
	b.DecompressionFileCountLimit = new(7)
	b.DecompressionSizeLimit = new("2GB")
	must.NotEqual(t, a, b)

	b = a.Copy()
	b.FilesystemIsolationExtraPaths[1] = "f:rx:/opt/bin/runme"
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
				HTTPReadTimeout:             new("30m"),
				HTTPMaxSize:                 new("100GB"),
				GCSTimeout:                  new("30m"),
				GitTimeout:                  new("30m"),
				HgTimeout:                   new("30m"),
				S3Timeout:                   new("30m"),
				DecompressionFileCountLimit: new(4096),
				DecompressionSizeLimit:      new("100GB"),
				DisableFilesystemIsolation:  new(false),
				FilesystemIsolationExtraPaths: []string{
					"f:r:/dev/urandom",
					"d:rx:/opt/bin",
					"d:r:/tmp/stash",
				},
				SetEnvironmentVariables: new(""),
			},
			other: &ArtifactConfig{
				HTTPReadTimeout:             new("5m"),
				HTTPMaxSize:                 new("2GB"),
				GCSTimeout:                  new("1m"),
				GitTimeout:                  new("2m"),
				HgTimeout:                   new("3m"),
				S3Timeout:                   new("4m"),
				DecompressionFileCountLimit: new(100),
				DecompressionSizeLimit:      new("8GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"d:rw:/opt/certs",
					"f:rx:/opt/bin/runme",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout:             new("5m"),
				HTTPMaxSize:                 new("2GB"),
				GCSTimeout:                  new("1m"),
				GitTimeout:                  new("2m"),
				HgTimeout:                   new("3m"),
				S3Timeout:                   new("4m"),
				DecompressionFileCountLimit: new(100),
				DecompressionSizeLimit:      new("8GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"d:rw:/opt/certs",
					"f:rx:/opt/bin/runme",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
		},
		{
			name:   "null source",
			source: nil,
			other: &ArtifactConfig{
				HTTPReadTimeout:             new("5m"),
				HTTPMaxSize:                 new("2GB"),
				GCSTimeout:                  new("1m"),
				GitTimeout:                  new("2m"),
				HgTimeout:                   new("3m"),
				S3Timeout:                   new("4m"),
				DecompressionFileCountLimit: new(100),
				DecompressionSizeLimit:      new("8GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"d:rw:/opt/certs",
					"f:rx:/opt/bin/runme",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout:             new("5m"),
				HTTPMaxSize:                 new("2GB"),
				GCSTimeout:                  new("1m"),
				GitTimeout:                  new("2m"),
				HgTimeout:                   new("3m"),
				S3Timeout:                   new("4m"),
				DecompressionFileCountLimit: new(100),
				DecompressionSizeLimit:      new("8GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"d:rw:/opt/certs",
					"f:rx:/opt/bin/runme",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
		},
		{
			name: "null other",
			source: &ArtifactConfig{
				HTTPReadTimeout:             new("30m"),
				HTTPMaxSize:                 new("100GB"),
				GCSTimeout:                  new("30m"),
				GitTimeout:                  new("30m"),
				HgTimeout:                   new("30m"),
				S3Timeout:                   new("30m"),
				DecompressionFileCountLimit: new(4096),
				DecompressionSizeLimit:      new("100GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"f:r:/dev/urandom",
					"d:rx:/opt/bin",
					"d:r:/tmp/stash",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
			other: nil,
			expected: &ArtifactConfig{
				HTTPReadTimeout:             new("30m"),
				HTTPMaxSize:                 new("100GB"),
				GCSTimeout:                  new("30m"),
				GitTimeout:                  new("30m"),
				HgTimeout:                   new("30m"),
				S3Timeout:                   new("30m"),
				DecompressionFileCountLimit: new(4096),
				DecompressionSizeLimit:      new("100GB"),
				DisableFilesystemIsolation:  new(true),
				FilesystemIsolationExtraPaths: []string{
					"f:r:/dev/urandom",
					"d:rx:/opt/bin",
					"d:r:/tmp/stash",
				},
				SetEnvironmentVariables: new("FOO,BAR"),
			},
		},
		{
			name: "null fsIsolationLocation",
			source: &ArtifactConfig{
				HTTPReadTimeout:               new("30m"),
				HTTPMaxSize:                   new("100GB"),
				GCSTimeout:                    new("30m"),
				GitTimeout:                    new("30m"),
				HgTimeout:                     new("30m"),
				S3Timeout:                     new("30m"),
				DecompressionFileCountLimit:   new(4096),
				DecompressionSizeLimit:        new("100GB"),
				DisableFilesystemIsolation:    new(false),
				FilesystemIsolationExtraPaths: nil,
				SetEnvironmentVariables:       new(""),
			},
			other: &ArtifactConfig{
				HTTPReadTimeout:               new("5m"),
				HTTPMaxSize:                   new("2GB"),
				GCSTimeout:                    new("1m"),
				GitTimeout:                    new("2m"),
				HgTimeout:                     new("3m"),
				S3Timeout:                     new("4m"),
				DecompressionFileCountLimit:   new(100),
				DecompressionSizeLimit:        new("8GB"),
				DisableFilesystemIsolation:    new(true),
				FilesystemIsolationExtraPaths: nil,
				SetEnvironmentVariables:       new("FOO,BAR"),
			},
			expected: &ArtifactConfig{
				HTTPReadTimeout:               new("5m"),
				HTTPMaxSize:                   new("2GB"),
				GCSTimeout:                    new("1m"),
				GitTimeout:                    new("2m"),
				HgTimeout:                     new("3m"),
				S3Timeout:                     new("4m"),
				DecompressionFileCountLimit:   new(100),
				DecompressionSizeLimit:        new("8GB"),
				DisableFilesystemIsolation:    new(true),
				FilesystemIsolationExtraPaths: nil,
				SetEnvironmentVariables:       new("FOO,BAR"),
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
				a.HTTPReadTimeout = new("invalid")
			},
			expErr: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = new("")
			},
			expErr: "http_read_timeout not a valid duration",
		},
		{
			name: "http read timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = new("0")
			},
			expErr: "",
		},
		{
			name: "http read timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPReadTimeout = new("-10m")
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
				a.HTTPMaxSize = new("invalid")
			},
			expErr: "http_max_size not a valid size",
		},
		{
			name: "http max size is empty",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = new("")
			},
			expErr: "http_max_size not a valid size",
		},
		{
			name: "http max size is zero",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = new("0")
			},
			expErr: "",
		},
		{
			name: "http max size is negative",
			config: func(a *ArtifactConfig) {
				a.HTTPMaxSize = new("-l0MB")
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
				a.GCSTimeout = new("invalid")
			},
			expErr: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = new("")
			},
			expErr: "gcs_timeout not a valid duration",
		},
		{
			name: "gcs timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = new("0")
			},
			expErr: "",
		},
		{
			name: "gcs timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GCSTimeout = new("-l0m")
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
				a.GitTimeout = new("invalid")
			},
			expErr: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is empty",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = new("")
			},
			expErr: "git_timeout not a valid duration",
		},
		{
			name: "git timeout is zero",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = new("0")
			},
			expErr: "",
		},
		{
			name: "git timeout is negative",
			config: func(a *ArtifactConfig) {
				a.GitTimeout = new("-l0m")
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
				a.HgTimeout = new("invalid")
			},
			expErr: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is empty",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = new("")
			},
			expErr: "hg_timeout not a valid duration",
		},
		{
			name: "hg timeout is zero",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = new("0")
			},
			expErr: "",
		},
		{
			name: "hg timeout is negative",
			config: func(a *ArtifactConfig) {
				a.HgTimeout = new("-l0m")
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
				a.S3Timeout = new("invalid")
			},
			expErr: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is empty",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = new("")
			},
			expErr: "s3_timeout not a valid duration",
		},
		{
			name: "s3 timeout is zero",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = new("0")
			},
			expErr: "",
		},
		{
			name: "s3 timeout is negative",
			config: func(a *ArtifactConfig) {
				a.S3Timeout = new("-l0m")
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
				a.DecompressionFileCountLimit = new(-1)
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
				a.DecompressionSizeLimit = new("-1GB")
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
			name: "fs isolation extra paths contains invalid path",
			config: func(a *ArtifactConfig) {
				a.FilesystemIsolationExtraPaths = []string{
					"f:r:/dev/urandom",
					"failure",
				}
			},
			expErr: "filesystem_isolation_extra_paths contains invalid lockdown path \"failure\"",
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
				must.ErrorContains(t, err, tc.expErr)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
