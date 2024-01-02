// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

// Consul represents optional per-group consul configuration.
type Consul struct {
	// Namespace in which to operate in Consul.
	Namespace string
}

// Copy the Consul block.
func (c *Consul) Copy() *Consul {
	if c == nil {
		return nil
	}
	return &Consul{
		Namespace: c.Namespace,
	}
}

// Equal returns whether c and o are the same.
func (c *Consul) Equal(o *Consul) bool {
	if c == nil || o == nil {
		return c == o
	}
	return c.Namespace == o.Namespace
}

// Validate returns whether c is valid.
func (c *Consul) Validate() error {
	// nothing to do here
	return nil
}

// ConsulUsage is provides meta information about how Consul is used by a job,
// noting which connect services and normal services will be registered, and
// whether the keystore will be read via template.
type ConsulUsage struct {
	Services []string
	KV       bool
}

// Used returns true if Consul is used for registering services or reading from
// the keystore.
func (cu *ConsulUsage) Used() bool {
	switch {
	case cu.KV:
		return true
	case len(cu.Services) > 0:
		return true
	}
	return false
}

// ConsulUsages returns a map from Consul namespace to things that will use Consul,
// including ConsulConnect TaskKinds, Consul Services from groups and tasks, and
// a boolean indicating if Consul KV is in use.
func (j *Job) ConsulUsages() map[string]*ConsulUsage {
	m := make(map[string]*ConsulUsage)

	for _, tg := range j.TaskGroups {
		namespace := j.ConsulNamespace
		if tgNamespace := tg.Consul.GetNamespace(); tgNamespace != "" {
			namespace = tgNamespace
		}
		if _, exists := m[namespace]; !exists {
			m[namespace] = new(ConsulUsage)
		}

		// Gather group services
		for _, service := range tg.Services {
			if service.Provider == ServiceProviderConsul {
				m[namespace].Services = append(m[namespace].Services, service.Name)
			}
		}

		// Gather task services and KV usage
		for _, task := range tg.Tasks {
			for _, service := range task.Services {
				if service.Provider == ServiceProviderConsul {
					m[namespace].Services = append(m[namespace].Services, service.Name)
				}
			}
			if len(task.Templates) > 0 {
				m[namespace].KV = true
			}
		}
	}

	return m
}
