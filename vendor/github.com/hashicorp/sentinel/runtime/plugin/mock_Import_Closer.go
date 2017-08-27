package plugin

import "io"

// MockImportCloser augments MockImport to also implement io.Closer
type MockImportCloser struct {
	MockImport
}

// Configure provides a mock function with given fields: _a0
func (_m *MockImportCloser) Close() error {
	ret := _m.Called()
	return ret.Error(0)
}

var _ io.Closer = (*MockImportCloser)(nil)
