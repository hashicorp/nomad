//go:build !ent
// +build !ent

package fsm

// allocQuota returns the quota object associated with the allocation. In
// anything but Premium this will always be empty
func (n *FSM) allocQuota(_ string) (string, error) { return "", nil }
