// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// Consul represents optional per-group consul configuration.
type Consul struct {
	// Namespace in which to operate in Consul.
	//
	// Namespace is set on the client and can be overriden by the consul.namespace
	// field in the jobspec.
	Namespace string

	// UseIdentity tells the server to sign identities for Consul. In Nomad 1.9+ this
	// field will be ignored (and treated as though it were set to true).
	//
	// UseIdentity is set on the server.
	UseIdentity bool

	// ServiceIdentity is intended to reduce overhead for jobspec authors and make
	// for graceful upgrades without forcing rewrite of all jobspecs. If set, when a
	// job has a service block with the “consul” provider, the Nomad server will sign
	// a Workload Identity for that service and add it to the service block. The
	// client will use this identity rather than the client's Consul token for the
	// group_service and envoy_bootstrap_hook.
	//
	// The name field of the identity is always set to
	// "consul-service/${service_name}-${service_port}".
	//
	// ServiceIdentity is set on the server.
	ServiceIdentity *WorkloadIdentity

	// TemplateIdentity is intended to reduce overhead for jobspec authors and make
	// for graceful upgrades without forcing rewrite of all jobspecs. If set, when a
	// job has both a template block and a consul block, the Nomad server will sign a
	// Workload Identity for that task. The client will use this identity rather than
	// the client's Consul token for the template hook.
	//
	// The name field of the identity is always set to "consul".
	//
	// TemplateIdentity is set on the server.
	TemplateIdentity *WorkloadIdentity
}

// Copy the Consul block.
func (c *Consul) Copy() *Consul {
	if c == nil {
		return nil
	}
	nc := new(Consul)
	*nc = *c

	nc.ServiceIdentity = c.ServiceIdentity.Copy()
	nc.TemplateIdentity = c.TemplateIdentity.Copy()

	return nc
}

// Equal returns whether c and o are the same.
func (c *Consul) Equal(o *Consul) bool {
	if c == nil || o == nil {
		return c == o
	}
	switch {
	case c.Namespace != o.Namespace:
		return false
	case !c.ServiceIdentity.Equal(o.ServiceIdentity):
		return false
	case !c.TemplateIdentity.Equal(o.TemplateIdentity):
		return false
	}
	return true
}

// Validate returns whether c is valid.
func (c *Consul) Validate() error {
	var mErr multierror.Error

	if c.UseIdentity && c.ServiceIdentity == nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("ServiceIdentity must be set if consul.use_identity is set"))
	}

	if c.ServiceIdentity != nil {
		if err := c.ServiceIdentity.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	if c.TemplateIdentity != nil {
		if err := c.TemplateIdentity.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
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
