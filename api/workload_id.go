package api

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks.
type WorkloadIdentity struct {
	Env  *bool `hcl:"env,optional"`
	File *bool `hcl:"file,optional"`
}
