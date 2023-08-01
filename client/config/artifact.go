package config

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// ArtifactConfig is the internal readonly copy of the client agent's
// ArtifactConfig.
type ArtifactConfig struct {
	HTTPReadTimeout time.Duration
	HTTPMaxBytes    int64

	GCSTimeout time.Duration
	GitTimeout time.Duration
	HgTimeout  time.Duration
	S3Timeout  time.Duration

	DecompressionLimitSize      int64
	DecompressionLimitFileCount int
}

// ArtifactConfigFromAgent creates a new internal readonly copy of the client
// agent's ArtifactConfig. The config should have already been validated.
func ArtifactConfigFromAgent(c *config.ArtifactConfig) (*ArtifactConfig, error) {
	newConfig := &ArtifactConfig{}

	t, err := time.ParseDuration(*c.HTTPReadTimeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTPReadTimeout: %w", err)
	}
	newConfig.HTTPReadTimeout = t

	s, err := humanize.ParseBytes(*c.HTTPMaxSize)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTPMaxSize: %w", err)
	}
	newConfig.HTTPMaxBytes = int64(s)

	t, err = time.ParseDuration(*c.GCSTimeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing GCSTimeout: %w", err)
	}
	newConfig.GCSTimeout = t

	t, err = time.ParseDuration(*c.GitTimeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing GitTimeout: %w", err)
	}
	newConfig.GitTimeout = t

	t, err = time.ParseDuration(*c.HgTimeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing HgTimeout: %w", err)
	}
	newConfig.HgTimeout = t

	t, err = time.ParseDuration(*c.S3Timeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing S3Timeout: %w", err)
	}
	newConfig.S3Timeout = t

	s, err = humanize.ParseBytes(*c.DecompressionSizeLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing DecompressionLimitSize: %w", err)
	}
	newConfig.DecompressionLimitSize = int64(s)

	// no parsing its just an int
	newConfig.DecompressionLimitFileCount = *c.DecompressionFileCountLimit

	return newConfig, nil
}

func (a *ArtifactConfig) Copy() *ArtifactConfig {
	if a == nil {
		return nil
	}

	newCopy := *a
	return &newCopy
}
