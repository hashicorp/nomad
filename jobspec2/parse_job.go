package jobspec2

import (
	"time"

	"github.com/hashicorp/nomad/api"
)

func normalizeJob(jc *jobConfig) {
	j := jc.Job
	if j.Name == nil {
		j.Name = &jc.JobID
	}
	if j.ID == nil {
		j.ID = &jc.JobID
	}

	if j.Periodic != nil && j.Periodic.Spec != nil {
		v := "cron"
		j.Periodic.SpecType = &v
	}

	normalizeVault(jc.Vault)

	if len(jc.Tasks) != 0 {
		alone := make([]*api.TaskGroup, 0, len(jc.Tasks))
		for _, t := range jc.Tasks {
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
				t.Vault = jc.Vault
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
