package getter

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/helper"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// parameters is encoded by the Nomad client and decoded by the getter sub-process
// so it can know what to do. We use standard IO instead of parameters variables
// because the job-submitter has control over the parameters and that is scary,
// see https://www.opencve.io/cve/CVE-2022-41716.
type parameters struct {
	// Config
	HTTPReadTimeout time.Duration `json:"http_read_timeout"`
	HTTPMaxBytes    int64         `json:"http_max_bytes"`
	GCSTimeout      time.Duration `json:"gcs_timeout"`
	GitTimeout      time.Duration `json:"git_timeout"`
	HgTimeout       time.Duration `json:"hg_timeout"`
	S3Timeout       time.Duration `json:"s3_timeout"`

	// Artifact
	Mode        getter.ClientMode   `json:"artifact_mode"`
	Source      string              `json:"artifact_source"`
	Destination string              `json:"artifact_destination"`
	Headers     map[string][]string `json:"artifact_headers"`

	// Task Environment
	TaskDir string `json:"task_dir"`
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
// sub-process forcefully terminates. The default is 1 hour, unless
// one or more getter configurations is set higher.
func (p *parameters) deadline() time.Duration {
	const minimum = 1 * time.Hour
	max := minimum
	max = helper.Max(max, p.HTTPReadTimeout)
	max = helper.Max(max, p.GCSTimeout)
	max = helper.Max(max, p.GitTimeout)
	max = helper.Max(max, p.HgTimeout)
	max = helper.Max(max, p.S3Timeout)
	return max
}

// executes returns true if go-getter will be used in a mode that
// requires the use of exec.
func (p *parameters) executes() bool {
	if strings.HasPrefix(p.Source, "git::") {
		return true
	}
	if strings.HasPrefix(p.Source, "hg::") {
		return true
	}
	return false
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
	case p.Mode != o.Mode:
		return false
	case p.Source != o.Source:
		return false
	case p.Destination != o.Destination:
		return false
	case p.TaskDir != o.TaskDir:
		return false
	case !maps.EqualFunc(p.Headers, o.Headers, eq):
		return false
	}

	return true
}

func eq(a, b []string) bool {
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
		Client: cleanhttp.DefaultClient(),
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
	return &getter.Client{
		Ctx:             ctx,
		Src:             p.Source,
		Dst:             p.Destination,
		Mode:            p.Mode,
		Umask:           umask,
		Insecure:        false,
		DisableSymlinks: true,
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
