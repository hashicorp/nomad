//go:build !ent

package nomad

func (c *LicenseConfig) Validate() error {
	return nil
}
