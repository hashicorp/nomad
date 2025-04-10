// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proclib

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
)

func MockWranglers(t testing.TB) *Wranglers {
	return &Wranglers{
		configs: &Configs{
			Logger: testlog.HCLogger(t),
		},
		m:      make(map[Task]ProcessWrangler),
		create: mocks,
	}
}

func mocks(Task) ProcessWrangler {
	return new(mock)
}

type mock struct {
}

func (m *mock) Initialize() error {
	return nil
}

func (m *mock) Kill() error {
	return nil
}

func (m *mock) Cleanup() error {
	return nil
}
