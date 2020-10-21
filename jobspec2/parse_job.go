package jobspec2

import (
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/nomad/api"
)

type jobWrapper struct {
	JobID string `hcl:",label"`
	Job   *api.Job

	Extra struct {
		Vault *api.Vault  `hcl:"vault,block"`
		Tasks []*api.Task `hcl:"task,block"`
	}
}

func decodeJob(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	m := val.(*jobWrapper)
	extra, _ := gohcl.ImpliedBodySchema(m.Extra)
	content, job, diags := body.PartialContent(extra)
	if len(diags) != 0 {
		return diags
	}

	for _, b := range content.Blocks {
		if b.Type == "vault" {
			v := &api.Vault{}
			diags = append(diags, hclDecoder.DecodeBody(b.Body, ctx, v)...)
			m.Extra.Vault = v
		} else if b.Type == "task" {
			t := &api.Task{}
			diags = append(diags, hclDecoder.DecodeBody(b.Body, ctx, t)...)
			if len(b.Labels) == 1 {
				t.Name = b.Labels[0]
				m.Extra.Tasks = append(m.Extra.Tasks, t)
			}
		}
	}

	m.Job = &api.Job{}
	return hclDecoder.DecodeBody(job, ctx, m.Job)
}

func normalizeJob(jw *jobWrapper) {
	j := jw.Job
	if j.Name == nil {
		j.Name = &jw.JobID
	}
	if j.ID == nil {
		j.ID = &jw.JobID
	}

	if j.Periodic != nil && j.Periodic.Spec != nil {
		v := "cron"
		j.Periodic.SpecType = &v
	}

	normalizeVault(jw.Extra.Vault)

	if len(jw.Extra.Tasks) != 0 {
		alone := make([]*api.TaskGroup, 0, len(jw.Extra.Tasks))
		for _, t := range jw.Extra.Tasks {
			alone = append(alone, &api.TaskGroup{
				Name:  &t.Name,
				Tasks: []*api.Task{t},
			})
		}
		alone = append(alone, j.TaskGroups...)
		j.TaskGroups = alone
	}

	for _, tg := range j.TaskGroups {
		normalizeNetworkPorts(tg.Networks)
		for _, t := range tg.Tasks {
			if t.Resources != nil {
				normalizeNetworkPorts(t.Resources.Networks)
			}

			normalizeTemplates(t.Templates)

			// normalize Vault
			normalizeVault(t.Vault)

			if t.Vault == nil {
				t.Vault = jw.Extra.Vault
			}
		}
	}
}

func normalizeVault(v *api.Vault) {
	if v == nil {
		return
	}

	if v.Env == nil {
		v.Env = boolToPtr(true)
	}
	if v.ChangeMode == nil {
		v.ChangeMode = stringToPtr("restart")
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

func int8ToPtr(v int8) *int8 {
	return &v
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
