package config

import (
	"fmt"
	"math"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/helper/pointer"
)

// ArtifactConfig is the configuration specific to the Artifact stanza
type ArtifactConfig struct {
	// HTTPReadTimeout is the duration in which a download must complete or
	// it will be canceled. Defaults to 30m.
	HTTPReadTimeout *string `hcl:"http_read_timeout"`

	// HTTPMaxSize is the maximum size of an artifact that will be downloaded.
	// Defaults to 100GB.
	HTTPMaxSize *string `hcl:"http_max_size"`

	// GCSTimeout is the duration in which a GCS operation must complete or
	// it will be canceled. Defaults to 30m.
	GCSTimeout *string `hcl:"gcs_timeout"`

	// GitTimeout is the duration in which a git operation must complete or
	// it will be canceled. Defaults to 30m.
	GitTimeout *string `hcl:"git_timeout"`

	// HgTimeout is the duration in which an hg operation must complete or
	// it will be canceled. Defaults to 30m.
	HgTimeout *string `hcl:"hg_timeout"`

	// S3Timeout is the duration in which an S3 operation must complete or
	// it will be canceled. Defaults to 30m.
	S3Timeout *string `hcl:"s3_timeout"`
}

func (a *ArtifactConfig) Copy() *ArtifactConfig {
	if a == nil {
		return nil
	}

	newCopy := &ArtifactConfig{}
	if a.HTTPReadTimeout != nil {
		newCopy.HTTPReadTimeout = pointer.Of(*a.HTTPReadTimeout)
	}
	if a.HTTPMaxSize != nil {
		newCopy.HTTPMaxSize = pointer.Of(*a.HTTPMaxSize)
	}
	if a.GCSTimeout != nil {
		newCopy.GCSTimeout = pointer.Of(*a.GCSTimeout)
	}
	if a.GitTimeout != nil {
		newCopy.GitTimeout = pointer.Of(*a.GitTimeout)
	}
	if a.HgTimeout != nil {
		newCopy.HgTimeout = pointer.Of(*a.HgTimeout)
	}
	if a.S3Timeout != nil {
		newCopy.S3Timeout = pointer.Of(*a.S3Timeout)
	}

	return newCopy
}

func (a *ArtifactConfig) Merge(o *ArtifactConfig) *ArtifactConfig {
	if a == nil {
		return o.Copy()
	}
	if o == nil {
		return a.Copy()
	}

	newCopy := a.Copy()
	if o.HTTPReadTimeout != nil {
		newCopy.HTTPReadTimeout = pointer.Of(*o.HTTPReadTimeout)
	}
	if o.HTTPMaxSize != nil {
		newCopy.HTTPMaxSize = pointer.Of(*o.HTTPMaxSize)
	}
	if o.GCSTimeout != nil {
		newCopy.GCSTimeout = pointer.Of(*o.GCSTimeout)
	}
	if o.GitTimeout != nil {
		newCopy.GitTimeout = pointer.Of(*o.GitTimeout)
	}
	if o.HgTimeout != nil {
		newCopy.HgTimeout = pointer.Of(*o.HgTimeout)
	}
	if o.S3Timeout != nil {
		newCopy.S3Timeout = pointer.Of(*o.S3Timeout)
	}

	return newCopy
}

func (a *ArtifactConfig) Validate() error {
	if a == nil {
		return fmt.Errorf("artifact must not be nil")
	}

	if a.HTTPReadTimeout == nil {
		return fmt.Errorf("http_read_timeout must be set")
	}
	if v, err := time.ParseDuration(*a.HTTPReadTimeout); err != nil {
		return fmt.Errorf("http_read_timeout not a valid duration: %w", err)
	} else if v < 0 {
		return fmt.Errorf("http_read_timeout must be > 0")
	}

	if a.HTTPMaxSize == nil {
		return fmt.Errorf("http_max_size must be set")
	}
	if v, err := humanize.ParseBytes(*a.HTTPMaxSize); err != nil {
		return fmt.Errorf("http_max_size not a valid size: %w", err)
	} else if v > math.MaxInt64 {
		return fmt.Errorf("http_max_size must be < %d but found %d", int64(math.MaxInt64), v)
	}

	if a.GCSTimeout == nil {
		return fmt.Errorf("gcs_timeout must be set")
	}
	if v, err := time.ParseDuration(*a.GCSTimeout); err != nil {
		return fmt.Errorf("gcs_timeout not a valid duration: %w", err)
	} else if v < 0 {
		return fmt.Errorf("gcs_timeout must be > 0")
	}

	if a.GitTimeout == nil {
		return fmt.Errorf("git_timeout must be set")
	}
	if v, err := time.ParseDuration(*a.GitTimeout); err != nil {
		return fmt.Errorf("git_timeout not a valid duration: %w", err)
	} else if v < 0 {
		return fmt.Errorf("git_timeout must be > 0")
	}

	if a.HgTimeout == nil {
		return fmt.Errorf("hg_timeout must be set")
	}
	if v, err := time.ParseDuration(*a.HgTimeout); err != nil {
		return fmt.Errorf("hg_timeout not a valid duration: %w", err)
	} else if v < 0 {
		return fmt.Errorf("hg_timeout must be > 0")
	}

	if a.S3Timeout == nil {
		return fmt.Errorf("s3_timeout must be set")
	}
	if v, err := time.ParseDuration(*a.S3Timeout); err != nil {
		return fmt.Errorf("s3_timeout not a valid duration: %w", err)
	} else if v < 0 {
		return fmt.Errorf("s3_timeout must be > 0")
	}

	return nil
}

func DefaultArtifactConfig() *ArtifactConfig {
	return &ArtifactConfig{
		// Read timeout for HTTP operations. Must be long enough to
		// accommodate large/slow downloads.
		HTTPReadTimeout: pointer.Of("30m"),

		// Maximum download size. Must be large enough to accommodate
		// large downloads.
		HTTPMaxSize: pointer.Of("100GB"),

		// Timeout for GCS operations. Must be long enough to
		// accommodate large/slow downloads.
		GCSTimeout: pointer.Of("30m"),

		// Timeout for Git operations. Must be long enough to
		// accommodate large/slow clones.
		GitTimeout: pointer.Of("30m"),

		// Timeout for Hg operations. Must be long enough to
		// accommodate large/slow clones.
		HgTimeout: pointer.Of("30m"),

		// Timeout for S3 operations. Must be long enough to
		// accommodate large/slow downloads.
		S3Timeout: pointer.Of("30m"),
	}
}
