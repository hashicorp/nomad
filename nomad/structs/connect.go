package structs

// ConsulConfigEntries represents Consul ConfigEntry definitions from a job.
type ConsulConfigEntries struct {
	Ingress     map[string]*ConsulIngressConfigEntry
	Terminating map[string]*ConsulTerminatingConfigEntry
	// Mesh later
}

// ConfigEntries accumulates the Consul Configuration Entries defined in task groups
// of j.
func (j *Job) ConfigEntries() *ConsulConfigEntries {
	entries := &ConsulConfigEntries{
		Ingress:     make(map[string]*ConsulIngressConfigEntry),
		Terminating: make(map[string]*ConsulTerminatingConfigEntry),
		// Mesh later
	}

	for _, tg := range j.TaskGroups {
		for _, service := range tg.Services {
			if service.Connect.IsGateway() {
				gateway := service.Connect.Gateway
				if ig := gateway.Ingress; ig != nil {
					entries.Ingress[service.Name] = ig
				} else if tg := gateway.Terminating; tg != nil {
					entries.Terminating[service.Name] = tg
				} // mesh later
			}
		}
	}

	return entries
}
