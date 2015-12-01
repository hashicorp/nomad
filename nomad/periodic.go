package nomad

import "github.com/hashicorp/nomad/nomad/structs"

type PeriodicRunner interface {
	SetEnabled(enabled bool)
	Add(job *structs.Job) error
	Remove(jobID string) error
	ForceRun(jobID string) error
}

type PeriodicDispatch struct{}

func (p *PeriodicDispatch) SetEnabled(enabled bool) {
}

func (p *PeriodicDispatch) Add(job *structs.Job) error {
	return nil
}

func (p *PeriodicDispatch) Remove(jobID string) error {
	return nil
}

func (p *PeriodicDispatch) ForceRun(jobID string) error {
	return nil
}
