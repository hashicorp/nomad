package allocrunner

import (
	"sync"
)

// taskCoordinatorController is used by the taskCoordinator to block or allow
// tasks from running.
type taskCoordinatorController struct {
	lock      sync.Mutex
	ch        chan struct{}
	updatedCh chan struct{}
	blocking  bool
}

// newTaskCoordinatorController returns a new taskCoordinatorController in the
// blocked state.
func newTaskCoordinatorController() *taskCoordinatorController {
	return &taskCoordinatorController{
		blocking:  true,
		ch:        make(chan struct{}),
		updatedCh: make(chan struct{}),
	}
}

// run starts the taskCoordinatorController and feeds the channel if the
// controller is not blocked.
func (c *taskCoordinatorController) run(shutdownCh <-chan struct{}) {
	for {
		if c.blocking {
			select {
			case <-shutdownCh:
				return
			case <-c.updatedCh:
				continue
			}
		} else {
			select {
			case c.ch <- struct{}{}:
			case <-shutdownCh:
				return
			case <-c.updatedCh:
				continue
			}
		}
	}
}

// waitCh returns a channel that can be used to determine if the caller should
// block and wait.
func (c *taskCoordinatorController) waitCh() <-chan struct{} {
	return c.ch
}

// allow is used to switch the controller to allow listeners to proceed.
func (c *taskCoordinatorController) allow() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.blocking {
		return
	}

	c.blocking = false
	c.updatedCh <- struct{}{}
}

// block is used to switch the controller to block listeners from proceeding.
func (c *taskCoordinatorController) block() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.blocking {
		return
	}

	c.blocking = true
	c.updatedCh <- struct{}{}
}
