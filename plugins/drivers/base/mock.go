package base

import "time"

// MockDriver is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockDriver struct {
	RecoverTaskF func(*TaskHandle) error
	StartTaskF   func(*TaskConfig) (*TaskHandle, error)
}

func (d *MockDriver) Fingerprint() *Fingerprint                                          { return nil }
func (d *MockDriver) RecoverTask(h *TaskHandle) error                                    { return d.RecoverTaskF(h) }
func (d *MockDriver) StartTask(c *TaskConfig) (*TaskHandle, error)                       { return d.StartTaskF(c) }
func (d *MockDriver) WaitTask(taskID string) chan *TaskResult                            { return nil }
func (d *MockDriver) StopTask(taskID string, timeout time.Duration, signal string) error { return nil }
func (d *MockDriver) DestroyTask(taskID string)                                          {}
func (d *MockDriver) ListTasks(*ListTasksQuery) ([]*TaskSummary, error)                  { return nil, nil }
func (d *MockDriver) InspectTask(taskID string) (*TaskStatus, error)                     { return nil, nil }
func (d *MockDriver) TaskStats(taskID string) (*TaskStats, error)                        { return nil, nil }
