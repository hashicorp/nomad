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
	infos    []mockArgs
	warnings []mockArgs
	errors   []mockArgs
	t        *testing.T
}

type mockArgs struct {
	winId uint32
	msg   string
}

func (m *MockEventlog) ExpectInfo(v1 WindowsEventId, v2 string) {
	m.infos = append(m.infos, mockArgs{uint32(v1), v2})
}

func (m *MockEventlog) ExpectWarning(v1 WindowsEventId, v2 string) {
	m.warnings = append(m.warnings, mockArgs{uint32(v1), v2})
}

func (m *MockEventlog) ExpectError(v1 WindowsEventId, v2 string) {
	m.errors = append(m.errors, mockArgs{uint32(v1), v2})
}

func (m *MockEventlog) Info(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.infos[0]
	m.infos = m.infos[1:]

	must.Eq(m.t, expectedArgs.winId, v1, must.Sprint("Incorrect WindowsEventId value"))
	must.Eq(m.t, expectedArgs.msg, v2, must.Sprint("Incorrect message value"))

	return nil
}

func (m *MockEventlog) Warning(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.warnings[0]
	m.warnings = m.warnings[1:]

	must.Eq(m.t, expectedArgs.winId, v1, must.Sprint("Incorrect WindowsEventId value"))
	must.Eq(m.t, expectedArgs.msg, v2, must.Sprint("Incorrect message value"))

	return nil
}

func (m *MockEventlog) Error(v1 uint32, v2 string) error {
	m.t.Helper()

	expectedArgs := m.errors[0]
	m.errors = m.errors[1:]

	must.Eq(m.t, expectedArgs.winId, v1, must.Sprint("Incorrect WindowsEventId value"))
	must.Eq(m.t, expectedArgs.msg, v2, must.Sprint("Incorrect message value"))

	return nil
}

func (m *MockEventlog) Close() error { return nil }

func (m *MockEventlog) AssertExpectations() {
	must.SliceEmpty(m.t, m.infos, must.Sprintf("Info expecting %d more invocations", len(m.infos)))
	must.SliceEmpty(m.t, m.warnings, must.Sprintf("Warning expecting %d more invocations", len(m.warnings)))
	must.SliceEmpty(m.t, m.errors, must.Sprintf("Error expecting %d more invocations", len(m.errors)))
}
