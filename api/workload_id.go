package api

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks.
type WorkloadIdentity struct {
	Env  *bool `hcl:"env,optional"`
	File *bool `hcl:"file,optional"`
}

func (wi *WorkloadIdentity) Canonicalize() {
	if wi == nil {
		return
	}
	if wi.Env == nil {
		wi.Env = pointerOf(true)
	}
	if wi.File == nil {
		wi.File = pointerOf(true)
	}
}
