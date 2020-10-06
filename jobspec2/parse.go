package jobspec2

import (
	"bytes"
	"io"
	"os"
	"time"

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

	normalizeJob(&result.Job)
	return &result.Job, nil
}

func normalizeJob(j *api.Job) {
	j.Name = j.ID
	if j.Periodic != nil && j.Periodic.Spec != nil {
		v := "cron"
		j.Periodic.SpecType = &v
	}

	for _, tg := range j.TaskGroups {
		normalizeNetworkPorts(tg.Networks)
		for _, t := range tg.Tasks {
			if t.Resources != nil {
				normalizeNetworkPorts(t.Resources.Networks)
			}

			normalizeTemplates(t.Templates)

			// normalize Vault
			if t.Vault != nil && t.Vault.Env == nil {
				t.Vault.Env = boolToPtr(true)
			}
			if t.Vault != nil && t.Vault.ChangeMode == nil {
				t.Vault.ChangeMode = stringToPtr("restart")
			}
		}
	}
}

func normalizeNetworkPorts(networks []*api.NetworkResource) {
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

func normalizeTemplates(templates []*api.Template) {
	if len(templates) == 0 {
		return
	}

	for _, t := range templates {
		if t.ChangeMode == nil {
			t.ChangeMode = stringToPtr("restart")
		}
		if t.Perms == nil {
			t.Perms = stringToPtr("0644")
		}
		if t.Splay == nil {
			t.Splay = durationToPtr(5 * time.Second)
		}
	}
}

func boolToPtr(v bool) *bool {
	return &v
}

func stringToPtr(v string) *string {
	return &v
}

func durationToPtr(v time.Duration) *time.Duration {
	return &v
}
