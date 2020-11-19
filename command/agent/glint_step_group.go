package agent

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/mitchellh/go-glint"
)

// glintStepGroup implements StepGroup with live updating and a display
// "window" for live terminal output (when using TermOutput).
type glintStepGroup struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	steps  []*glintStep
	closed bool
}

// Start a step in the output
func (f *glintStepGroup) Add(str string, args ...interface{}) Step {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Build our step
	step := &glintStep{ctx: f.ctx, status: newGlintStatus()}

	// Setup initial status
	step.Update(str, args...)

	// If we're closed we don't add this step to our waitgroup or document.
	// We still create a step and return a non-nil step so downstreams don't
	// crash.
	if !f.closed {
		// Add since we have a step
		step.wg = &f.wg
		f.wg.Add(1)

		// Add it to our list
		f.steps = append(f.steps, step)
	}

	return step
}

func (f *glintStepGroup) Wait() {
	f.mu.Lock()
	f.closed = true
	f.cancel()
	wg := &f.wg
	f.mu.Unlock()

	wg.Wait()
}

func (f *glintStepGroup) Body(context.Context) glint.Component {
	f.mu.Lock()
	defer f.mu.Unlock()

	var cs []glint.Component
	for _, s := range f.steps {
		cs = append(cs, s)
	}

	return glint.Fragment(cs...)
}

type glintStep struct {
	mu        sync.Mutex
	ctx       context.Context
	wg        *sync.WaitGroup
	done      bool
	msg       string
	statusVal string
	status    *glintStatus
	term      *glintTerm
}

func (f *glintStep) TermOutput() io.Writer {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.term == nil {
		t, err := newGlintTerm(f.ctx, 10, 80)
		if err != nil {
			panic(err)
		}

		f.term = t
	}

	return f.term
}

func (f *glintStep) Update(str string, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msg = fmt.Sprintf(str, args...)
	f.status.reset()

	if f.statusVal != "" {
		f.status.Step(f.statusVal, f.msg)
	} else {
		f.status.Update(f.msg)
	}
}

func (f *glintStep) Status(status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusVal = status
	f.status.reset()
	f.status.Step(status, f.msg)
}

func (f *glintStep) Done() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return
	}

	// Set done
	f.done = true

	// Set status
	if f.statusVal == "" {
		f.status.reset()
		f.status.Step(StatusOK, f.msg)
	}

	// Unset the waitgroup
	f.wg.Done()
}

func (f *glintStep) Abort() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return
	}

	f.done = true

	// This will cause the term to render the full scrollback from now on
	if f.term != nil {
		f.term.showFull()
	}

	f.status.Step(StatusError, f.msg)
	f.wg.Done()
}

func (f *glintStep) Body(context.Context) glint.Component {
	f.mu.Lock()
	defer f.mu.Unlock()

	var cs []glint.Component
	cs = append(cs, f.status)

	// If we have a terminal, output that too.
	if f.term != nil {
		cs = append(cs, f.term)
	}

	return glint.Fragment(cs...)
}
