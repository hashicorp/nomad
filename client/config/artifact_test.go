package config

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestArtifactConfigFromAgent(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name          string
		config        *config.ArtifactConfig
		expected      *ArtifactConfig
		expectedError string
	}{
		{
			name:   "from default",
			config: config.DefaultArtifactConfig(),
			expected: &ArtifactConfig{
				HTTPReadTimeout: 30 * time.Minute,
				HTTPMaxBytes:    100_000_000_000,
				GCSTimeout:      30 * time.Minute,
				GitTimeout:      30 * time.Minute,
				HgTimeout:       30 * time.Minute,
				S3Timeout:       30 * time.Minute,
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
			expectedError: "error parsing HTTPReadTimeout",
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
			expectedError: "error parsing HTTPMaxSize",
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
			expectedError: "error parsing GCSTimeout",
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
			expectedError: "error parsing GitTimeout",
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
			expectedError: "error parsing HgTimeout",
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
			expectedError: "error parsing S3Timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ArtifactConfigFromAgent(tc.config)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestArtifactConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	config := &ArtifactConfig{
		HTTPReadTimeout: time.Minute,
		HTTPMaxBytes:    1000,
		GCSTimeout:      2 * time.Minute,
		GitTimeout:      time.Second,
		HgTimeout:       time.Hour,
		S3Timeout:       5 * time.Minute,
	}

	// make sure values are copied.
	configCopy := config.Copy()
	require.Equal(t, config, configCopy)

	// modify copy and make sure original doesn't change.
	configCopy.HTTPReadTimeout = 5 * time.Minute
	configCopy.HTTPMaxBytes = 2000
	configCopy.GCSTimeout = 5 * time.Second
	configCopy.GitTimeout = 3 * time.Second
	configCopy.HgTimeout = 2 * time.Hour
	configCopy.S3Timeout = 10 * time.Minute

	require.Equal(t, &ArtifactConfig{
		HTTPReadTimeout: time.Minute,
		HTTPMaxBytes:    1000,
		GCSTimeout:      2 * time.Minute,
		GitTimeout:      time.Second,
		HgTimeout:       time.Hour,
		S3Timeout:       5 * time.Minute,
	}, config)
}
