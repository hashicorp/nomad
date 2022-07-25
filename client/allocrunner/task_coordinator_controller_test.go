package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestTaskCoordinatorController(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name string
		test func(*testing.T, *taskCoordinatorController)
	}{
		{
			name: "starts blocked",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				requireChannelBlocking(t, ctrl.waitCh(), "wait")
			},
		},
		{
			name: "block",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				ctrl.block()
				requireChannelBlocking(t, ctrl.waitCh(), "wait")
			},
		},
		{
			name: "allow",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				ctrl.allow()
				requireChannelPassing(t, ctrl.waitCh(), "wait")
			},
		},
		{
			name: "block twice",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				ctrl.block()
				ctrl.block()
				requireChannelBlocking(t, ctrl.waitCh(), "wait")
			},
		},
		{
			name: "allow twice",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				ctrl.allow()
				ctrl.allow()
				requireChannelPassing(t, ctrl.waitCh(), "wait")
			},
		},
		{
			name: "allow block allow",
			test: func(t *testing.T, ctrl *taskCoordinatorController) {
				ctrl.allow()
				requireChannelPassing(t, ctrl.waitCh(), "first allow")
				ctrl.block()
				requireChannelBlocking(t, ctrl.waitCh(), "block")
				ctrl.allow()
				requireChannelPassing(t, ctrl.waitCh(), "second allow")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := newTaskCoordinatorController()

			shutdownCh := make(chan struct{})
			defer close(shutdownCh)
			go ctrl.run(shutdownCh)
			tc.test(t, ctrl)
		})
	}
}
