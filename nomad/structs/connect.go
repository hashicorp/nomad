// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

// ConsulConfigEntries represents Consul ConfigEntry definitions from a job for
// a single Consul namespace.
type ConsulConfigEntries struct {
	Ingress     map[string]*ConsulIngressConfigEntry
	Terminating map[string]*ConsulTerminatingConfigEntry
}

// ConfigEntries accumulates the Consul Configuration Entries defined in task groups
// of j, organized by Consul namespace.
func (j *Job) ConfigEntries() map[string]*ConsulConfigEntries {
	collection := make(map[string]*ConsulConfigEntries)

	for _, tg := range j.TaskGroups {

		// accumulate config entries by namespace
		ns := tg.Consul.GetNamespace()
		if _, exists := collection[ns]; !exists {
			collection[ns] = &ConsulConfigEntries{
				Ingress:     make(map[string]*ConsulIngressConfigEntry),
				Terminating: make(map[string]*ConsulTerminatingConfigEntry),
			}
		}

		for _, service := range tg.Services {
			if service.Connect.IsGateway() {
				gateway := service.Connect.Gateway
				if ig := gateway.Ingress; ig != nil {
					collection[ns].Ingress[service.Name] = ig
				} else if term := gateway.Terminating; term != nil {
					collection[ns].Terminating[service.Name] = term
				}
			}
		}
	}

	return collection
}
