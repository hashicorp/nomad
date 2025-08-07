// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func NewMockEventlog(t *testing.T) *MockEventlog {
	return &MockEventlog{
		t: t,
	}
}

type MockEventlog struct {
	infos    [][2]any
	warnings [][2]any
	errors   [][2]any
	t        *testing.T
}

func (m *MockEventlog) ExpectInfo(v1 uint32, v2 string) {
	m.infos = append(m.infos, [2]any{v1, v2})
}

func (m *MockEventlog) ExpectWarning(v1 uint32, v2 string) {
	m.warnings = append(m.warnings, [2]any{v1, v2})
}

func (m *MockEventlog) ExpectError(v1 uint32, v2 string) {
	m.errors = append(m.errors, [2]any{v1, v2})
}

func (m *MockEventlog) Info(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.infos[0]
	m.infos = m.infos[1:]
	must.Eq(m.t, 2, len(expectedArgs),
		must.Sprint("Invalid number of expected arguments for Info"))
	must.Eq(m.t, expectedArgs[0].(uint32), v1,
		must.Sprint("Info received incorrect argument"))
	must.Eq(m.t, expectedArgs[1].(string), v2,
		must.Sprint("Info received incorrect argument"))
	return nil
}

func (m *MockEventlog) Warning(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.warnings[0]
	m.warnings = m.warnings[1:]
	must.Eq(m.t, 2, len(expectedArgs),
		must.Sprint("Invalid number of expected arguments for Warning"))
	must.Eq(m.t, expectedArgs[0].(uint32), v1,
		must.Sprint("Warning received incorrect argument"))
	must.Eq(m.t, expectedArgs[1].(string), v2,
		must.Sprint("Warning received incorrect argument"))

	return nil
}

func (m *MockEventlog) Error(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.errors[0]
	m.errors = m.errors[1:]
	must.Eq(m.t, 2, len(expectedArgs),
		must.Sprint("Invalid number of expected arguments for Error"))
	must.Eq(m.t, expectedArgs[0].(uint32), v1,
		must.Sprint("Error received incorrect argument"))
	must.Eq(m.t, expectedArgs[1].(string), v2,
		must.Sprint("Error received incorrect argument"))

	return nil
}

func (m *MockEventlog) Close() error { return nil }

func (m *MockEventlog) AssertExpectations() {
	must.SliceEmpty(m.t, m.infos, must.Sprintf("Info expecting %d more invocations", len(m.infos)))
	must.SliceEmpty(m.t, m.warnings, must.Sprintf("Warning expecting %d more invocations", len(m.warnings)))
	must.SliceEmpty(m.t, m.errors, must.Sprintf("Error expecting %d more invocations", len(m.errors)))
}
