// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/helper"
)

// parameters is encoded by the Nomad client and decoded by the getter sub-process
// so it can know what to do. We use standard IO to pass configuration to achieve
// better control over input sanitization risks.
// e.g. https://www.opencve.io/cve/CVE-2022-41716
type parameters struct {
	// Config
	HTTPReadTimeout               time.Duration `json:"http_read_timeout"`
	HTTPMaxBytes                  int64         `json:"http_max_bytes"`
	GCSTimeout                    time.Duration `json:"gcs_timeout"`
	GitTimeout                    time.Duration `json:"git_timeout"`
	HgTimeout                     time.Duration `json:"hg_timeout"`
	S3Timeout                     time.Duration `json:"s3_timeout"`
	DecompressionLimitFileCount   int           `json:"decompression_limit_file_count"`
	DecompressionLimitSize        int64         `json:"decompression_limit_size"`
	DisableFilesystemIsolation    bool          `json:"disable_filesystem_isolation"`
	FilesystemIsolationExtraPaths []string      `json:"filesystem_isolation_extra_paths"`
	SetEnvironmentVariables       string        `json:"set_environment_variables"`

	// Artifact
	Mode        getter.ClientMode   `json:"artifact_mode"`
	Insecure    bool                `json:"artifact_insecure"`
	Source      string              `json:"artifact_source"`
	Destination string              `json:"artifact_destination"`
	Headers     map[string][]string `json:"artifact_headers"`

	// Task Filesystem
	AllocDir string `json:"alloc_dir"`
	TaskDir  string `json:"task_dir"`
	User     string `json:"user"`
	Chown    bool   `json:"chown"`
}

func (p *parameters) reader() io.Reader {
	b, err := json.Marshal(p)
	if err != nil {
		b = nil
	}
	return strings.NewReader(string(b))
}

func (p *parameters) read(r io.Reader) error {
	return json.NewDecoder(r).Decode(p)
}

// deadline returns an absolute deadline before the artifact download
// sub-process forcefully terminates. The default is 1/2 hour, unless one or
// more getter configurations is set higher. A 1 minute grace period is added
// so that an internal timeout has a moment to complete before the process is
// terminated via signal.
func (p *parameters) deadline() time.Duration {
	const minimum = 30 * time.Minute
	maximum := minimum
	maximum = max(maximum, p.HTTPReadTimeout)
	maximum = max(maximum, p.GCSTimeout)
	maximum = max(maximum, p.GitTimeout)
	maximum = max(maximum, p.HgTimeout)
	maximum = max(maximum, p.S3Timeout)
	return maximum + 1*time.Minute
}

// Equal returns whether p and o are the same.
func (p *parameters) Equal(o *parameters) bool {
	if p == nil || o == nil {
		return p == o
	}

	switch {
	case p.HTTPReadTimeout != o.HTTPReadTimeout:
		return false
	case p.HTTPMaxBytes != o.HTTPMaxBytes:
		return false
	case p.GCSTimeout != o.GCSTimeout:
		return false
	case p.GitTimeout != o.GitTimeout:
		return false
	case p.HgTimeout != o.HgTimeout:
		return false
	case p.S3Timeout != o.S3Timeout:
		return false
	case p.DecompressionLimitFileCount != o.DecompressionLimitFileCount:
		return false
	case p.DecompressionLimitSize != o.DecompressionLimitSize:
		return false
	case p.DisableFilesystemIsolation != o.DisableFilesystemIsolation:
		return false
	case !helper.SliceSetEq(p.FilesystemIsolationExtraPaths, o.FilesystemIsolationExtraPaths):
		return false
	case p.SetEnvironmentVariables != o.SetEnvironmentVariables:
		return false
	case p.Mode != o.Mode:
		return false
	case p.Insecure != o.Insecure:
		return false
	case p.Source != o.Source:
		return false
	case p.Destination != o.Destination:
		return false
	case p.TaskDir != o.TaskDir:
		return false
	case !maps.EqualFunc(p.Headers, o.Headers, headersCompareFn):
		return false
	}

	return true
}

func headersCompareFn(a []string, b []string) bool {
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

const (
	// stop privilege escalation via setuid/setgid
	// https://github.com/hashicorp/nomad/issues/6176
	umask = fs.ModeSetuid | fs.ModeSetgid
)

func (p *parameters) client(ctx context.Context) *getter.Client {
	httpGetter := &getter.HttpGetter{
		Netrc:  true,
		Header: p.Headers,

		// Do not support the custom X-Terraform-Get header and
		// associated logic.
		XTerraformGetDisabled: true,

		// Disable HEAD requests as they can produce corrupt files when
		// retrying a download of a resource that has changed.
		// hashicorp/go-getter#219
		DoNotCheckHeadFirst: true,

		// Read timeout for HTTP operations. Must be long enough to
		// accommodate large/slow downloads.
		ReadTimeout: p.HTTPReadTimeout,

		// Maximum download size. Must be large enough to accommodate
		// large downloads.
		MaxBytes: p.HTTPMaxBytes,
	}

	// setup custom decompressors with file count and total size limits
	decompressors := getter.LimitedDecompressors(
		p.DecompressionLimitFileCount,
		p.DecompressionLimitSize,
	)

	return &getter.Client{
		Ctx:             ctx,
		Src:             p.Source,
		Dst:             p.Destination,
		Mode:            p.Mode,
		Insecure:        p.Insecure,
		Umask:           umask,
		DisableSymlinks: true,
		Decompressors:   decompressors,
		Getters: map[string]getter.Getter{
			"git": &getter.GitGetter{
				Timeout: p.GitTimeout,
			},
			"hg": &getter.HgGetter{
				Timeout: p.HgTimeout,
			},
			"gcs": &getter.GCSGetter{
				Timeout: p.GCSTimeout,
			},
			"s3": &getter.S3Getter{
				Timeout: p.S3Timeout,
			},
			"http":  httpGetter,
			"https": httpGetter,
		},
	}
}
