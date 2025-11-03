// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

type MockTaskStateUpdater struct {
	ch chan struct{}
}

func NewMockTaskStateUpdater() *MockTaskStateUpdater {
	return &MockTaskStateUpdater{
		ch: make(chan struct{}, 1),
	}
}

func (m *MockTaskStateUpdater) TaskStateUpdated() {
	select {
	case m.ch <- struct{}{}:
	default:
	}
}
