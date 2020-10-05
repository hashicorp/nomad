package jobspec2

import (
	"bytes"
	"io"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad/api"
)

func Parse(r io.Reader) (*api.Job, error) {
	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	evalContext := &hcl.EvalContext{}
	var result struct {
		Job api.Job `hcl:"job,block"`
	}
	err := hclsimple.Decode("job.hcl", buf.Bytes(), evalContext, &result)
	if err != nil {
		return nil, err
	}

	return &result.Job, nil
}
