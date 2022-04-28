// +build !ent

package structs

func (c *Consul) GetNamespace() string {
	return ""
}
