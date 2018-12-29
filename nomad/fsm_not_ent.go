// +build !ent

package nomad

// allocQuota returns the quota object associated with the allocation. In
// anything but Premium this will always be empty
func (n *nomadFSM) allocQuota(allocID string) (string, error) {
	return "", nil
}
