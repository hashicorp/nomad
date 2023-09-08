// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-getter"
	"github.com/shoenig/test/must"
)

const paramsAsJSON = `
{
  "http_read_timeout": 1000000000,
  "http_max_bytes": 2000,
  "gcs_timeout": 2000000000,
  "git_timeout": 3000000000,
  "hg_timeout": 4000000000,
  "s3_timeout": 5000000000,
  "decompression_limit_file_count": 3,
  "decompression_limit_size": 98765,
  "disable_filesystem_isolation": true,
  "set_environment_variables": "",
  "artifact_mode": 2,
  "artifact_source": "https://example.com/file.txt",
  "artifact_destination": "local/out.txt",
  "artifact_headers": {
    "X-Nomad-Artifact": ["hi"]
  },
  "alloc_dir": "/path/to/alloc",
  "task_dir": "/path/to/alloc/task"
}`

var paramsAsStruct = &parameters{
	HTTPReadTimeout:             1 * time.Second,
	HTTPMaxBytes:                2000,
	GCSTimeout:                  2 * time.Second,
	GitTimeout:                  3 * time.Second,
	HgTimeout:                   4 * time.Second,
	S3Timeout:                   5 * time.Second,
	DecompressionLimitFileCount: 3,
	DecompressionLimitSize:      98765,
	DisableFilesystemIsolation:  true,

	Mode:        getter.ClientModeFile,
	Source:      "https://example.com/file.txt",
	Destination: "local/out.txt",
	AllocDir:    "/path/to/alloc",
	TaskDir:     "/path/to/alloc/task",
	Headers: map[string][]string{
		"X-Nomad-Artifact": {"hi"},
	},
}

func TestParameters_reader(t *testing.T) {
	p := paramsAsStruct
	reader := p.reader()
	b, err := io.ReadAll(reader)
	must.NoError(t, err)
	must.EqJSON(t, paramsAsJSON, string(b))
}

func TestParameters_read(t *testing.T) {
	reader := strings.NewReader(paramsAsJSON)
	p := new(parameters)
	err := p.read(reader)
	must.NoError(t, err)
	must.Equal(t, paramsAsStruct, p)
}

func TestParameters_deadline(t *testing.T) {
	t.Run("typical", func(t *testing.T) {
		dur := paramsAsStruct.deadline()
		must.Eq(t, 31*time.Minute, dur)
	})

	t.Run("long", func(t *testing.T) {
		params := &parameters{
			HTTPReadTimeout: 1 * time.Hour,
			GCSTimeout:      2 * time.Hour,
			GitTimeout:      3 * time.Hour,
			HgTimeout:       4 * time.Hour,
			S3Timeout:       5 * time.Hour,
		}
		dur := params.deadline()
		must.Eq(t, 5*time.Hour+1*time.Minute, dur)
	})
}

func TestParameters_client(t *testing.T) {
	ctx := context.Background()
	c := paramsAsStruct.client(ctx)
	must.NotNil(t, c)

	// security options
	must.False(t, c.Insecure)
	must.True(t, c.DisableSymlinks)
	must.Eq(t, umask, c.Umask)

	// artifact options
	must.Eq(t, "https://example.com/file.txt", c.Src)
	must.Eq(t, "local/out.txt", c.Dst)

	// decompression limits
	const fileCountLimit = 3
	const fileSizeLimit = 98765
	must.Eq(t, fileSizeLimit, c.Decompressors["zip"].(*getter.ZipDecompressor).FileSizeLimit)
	must.Eq(t, fileCountLimit, c.Decompressors["zip"].(*getter.ZipDecompressor).FilesLimit)
	must.Eq(t, fileSizeLimit, c.Decompressors["tar.gz"].(*getter.TarGzipDecompressor).FileSizeLimit)
	must.Eq(t, fileCountLimit, c.Decompressors["tar.gz"].(*getter.TarGzipDecompressor).FilesLimit)
	must.Eq(t, fileSizeLimit, c.Decompressors["xz"].(*getter.XzDecompressor).FileSizeLimit)
	// xz does not support files count limit
}

func TestParameters_Equal_headers(t *testing.T) {
	p1 := &parameters{
		Headers: map[string][]string{
			"East": []string{"New York", "Florida"},
			"West": []string{"California"},
		},
	}

	p2 := &parameters{
		Headers: map[string][]string{
			"East": []string{"New York", "Florida"},
			"West": []string{"California"},
		},
	}

	// equal
	must.Equal(t, p1, p2)

	// equal
	must.Equal(t, p1, &parameters{
		Headers: map[string][]string{
			"West": {"California"},
			"East": {"Florida", "New York"},
		},
	})

	// not equal
	p2.Headers["East"] = []string{"New York"}
	must.NotEqual(t, p1, p2)
}
