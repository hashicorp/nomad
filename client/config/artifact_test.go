// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func TestArtifactConfigFromAgent(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		config *config.ArtifactConfig
		exp    *ArtifactConfig
		expErr string
	}{
		{
			name:   "from default",
			config: config.DefaultArtifactConfig(),
			exp: &ArtifactConfig{
				HTTPReadTimeout:             30 * time.Minute,
				HTTPMaxBytes:                100_000_000_000,
				GCSTimeout:                  30 * time.Minute,
				GitTimeout:                  30 * time.Minute,
				HgTimeout:                   30 * time.Minute,
				S3Timeout:                   30 * time.Minute,
				DecompressionLimitFileCount: 4096,
				DecompressionLimitSize:      100_000_000_000,
			},
		},
		{
			name: "invalid http read timeout",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("invalid"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			expErr: "error parsing HTTPReadTimeout",
		},
		{
			name: "invalid http max size",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("invalid"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			expErr: "error parsing HTTPMaxSize",
		},
		{
			name: "invalid gcs timeout",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("invalid"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			expErr: "error parsing GCSTimeout",
		},
		{
			name: "invalid git timeout",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("invalid"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("30m"),
			},
			expErr: "error parsing GitTimeout",
		},
		{
			name: "invalid hg timeout",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("invalid"),
				S3Timeout:       pointer.Of("30m"),
			},
			expErr: "error parsing HgTimeout",
		},
		{
			name: "invalid s3 timeout",
			config: &config.ArtifactConfig{
				HTTPReadTimeout: pointer.Of("30m"),
				HTTPMaxSize:     pointer.Of("100GB"),
				GCSTimeout:      pointer.Of("30m"),
				GitTimeout:      pointer.Of("30m"),
				HgTimeout:       pointer.Of("30m"),
				S3Timeout:       pointer.Of("invalid"),
			},
			expErr: "error parsing S3Timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ArtifactConfigFromAgent(tc.config)

			if tc.expErr != "" {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.expErr)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.exp, got)
			}
		})
	}
}

func TestArtifactConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	ac := &ArtifactConfig{
		HTTPReadTimeout:            time.Minute,
		HTTPMaxBytes:               1000,
		GCSTimeout:                 2 * time.Minute,
		GitTimeout:                 time.Second,
		HgTimeout:                  time.Hour,
		S3Timeout:                  5 * time.Minute,
		DisableFilesystemIsolation: true,
		SetEnvironmentVariables:    "FOO,BAR",
	}

	// make sure values are copied.
	configCopy := ac.Copy()
	must.Eq(t, ac, configCopy)

	// modify copy and make sure original doesn't change.
	configCopy.HTTPReadTimeout = 5 * time.Minute
	configCopy.HTTPMaxBytes = 2000
	configCopy.GCSTimeout = 5 * time.Second
	configCopy.GitTimeout = 3 * time.Second
	configCopy.HgTimeout = 2 * time.Hour
	configCopy.S3Timeout = 10 * time.Minute
	configCopy.DisableFilesystemIsolation = false
	configCopy.SetEnvironmentVariables = "BAZ"

	must.Eq(t, &ArtifactConfig{
		HTTPReadTimeout:            time.Minute,
		HTTPMaxBytes:               1000,
		GCSTimeout:                 2 * time.Minute,
		GitTimeout:                 time.Second,
		HgTimeout:                  time.Hour,
		S3Timeout:                  5 * time.Minute,
		DisableFilesystemIsolation: true,
		SetEnvironmentVariables:    "FOO,BAR",
	}, ac)
}
