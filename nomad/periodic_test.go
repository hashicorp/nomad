package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type MockPeriodic struct {
	Enabled bool
	Jobs    map[string]*structs.Job
}

func NewMockPeriodic() *MockPeriodic {
	return &MockPeriodic{Jobs: make(map[string]*structs.Job)}
}

func (m *MockPeriodic) SetEnabled(enabled bool) {
	m.Enabled = enabled
}

func (m *MockPeriodic) Add(job *structs.Job) error {
	if job == nil {
		return fmt.Errorf("Must pass non nil job")
	}

	m.Jobs[job.ID] = job
	return nil
}

func (m *MockPeriodic) Remove(jobID string) error {
	delete(m.Jobs, jobID)
	return nil
}

func (m *MockPeriodic) ForceRun(jobID string) error {
	return nil
}
