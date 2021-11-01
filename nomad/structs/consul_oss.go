//go:build !ent
// +build !ent

package structs

import "errors"

func (c *Consul) GetNamespace() string {
	return ""
}

func (c *Consul) Validate() error {
	if c.Namespace != "" {
		return errors.New("Setting Consul namespaces in a job requires Nomad Enterprise.")
	}
	return nil
}
