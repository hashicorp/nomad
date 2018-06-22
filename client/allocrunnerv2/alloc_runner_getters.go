package allocrunnerv2

import "github.com/hashicorp/nomad/nomad/structs"

func (ar *allocRunner) ID() string {
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()
	return ar.alloc.ID
}

func (ar *allocRunner) Alloc() *structs.Allocation {
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()
	return ar.alloc
}
