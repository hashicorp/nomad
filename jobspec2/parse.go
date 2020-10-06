package jobspec2

import (
	"bytes"
	"io"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
)

func Parse(r io.Reader) (*api.Job, error) {
	filename := "job.hcl"
	if f, ok := r.(*os.File); ok {
		filename = f.Name()
	}
	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	evalContext := &hcl.EvalContext{
		UnknownVariable: func(expr string) (cty.Value, error) {
			v := "${" + expr + "}"
			return cty.StringVal(v), nil
		},
	}
	var result struct {
		Job api.Job `hcl:"job,block"`
	}
	err := hclsimple.Decode(filename, buf.Bytes(), evalContext, &result)
	if err != nil {
		return nil, err
	}

	result.Job.Name = result.Job.ID
	if result.Job.Periodic != nil && result.Job.Periodic.Spec != nil {
		v := "cron"
		result.Job.Periodic.SpecType = &v
	}

	normalizeJob(&result.Job)
	return &result.Job, nil
}

func normalizeJob(j *api.Job) {
	for _, tg := range j.TaskGroups {
		parseDynamic(tg.Networks)
		for _, t := range tg.Tasks {
			if t.Resources != nil {
				parseDynamic(t.Resources.Networks)
			}
		}
	}
}

func parseDynamic(networks []*api.NetworkResource) {
	if networks == nil {
		return
	}
	for _, n := range networks {
		if len(n.DynamicPorts) == 0 {
			continue
		}

		dynamic := make([]api.Port, 0, len(n.DynamicPorts))
		var reserved []api.Port

		for _, p := range n.DynamicPorts {
			if p.Value > 0 {
				reserved = append(reserved, p)
			} else {
				dynamic = append(dynamic, p)
			}
		}
		if len(dynamic) == 0 {
			dynamic = nil
		}

		n.DynamicPorts = dynamic
		n.ReservedPorts = reserved
	}

}
