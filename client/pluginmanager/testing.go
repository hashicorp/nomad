package pluginmanager

import "context"

type MockPluginManager struct {
	RunF                      func()
	ShutdownF                 func()
	WaitForFirstFingerprintCh <-chan struct{}
}

func (m *MockPluginManager) Run()               { m.RunF() }
func (m *MockPluginManager) Shutdown()          { m.ShutdownF() }
func (m *MockPluginManager) PluginType() string { return "mock" }
func (m *MockPluginManager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	if m.WaitForFirstFingerprintCh != nil {
		return m.WaitForFirstFingerprintCh
	}

	ch := make(chan struct{})
	close(ch)
	return ch
}
