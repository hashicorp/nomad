package components

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/hashicorp/nomad/cli/icons"
	"github.com/mitchellh/go-glint"
)

func IconWarning() glint.Component {
	return Warning(icons.Warning + " ")
}

func IconFailure() glint.Component {
	return Error(icons.Failure + " ")
}

func IconSuccess() glint.Component {
	return Success(icons.Success + " ")
}

func IconHealthy() glint.Component {
	return Success(icons.Healthy + " ")
}

// A stateless variation of the Spinner that comes with go-glint
// This still has to be a struct with a Body method or else it doesn't
// rerender
func IconRunning() *IconRunningComponent {
	return &IconRunningComponent{}
}

const (
	stateRunning = 0
	stateDone    = 1
)

type IconRunningComponent struct {
	finished uint32
	final    glint.Component
}

func (c *IconRunningComponent) Body(context.Context) glint.Component {
	if atomic.LoadUint32(&c.finished) == stateDone {
		return IconSuccess()
	}

	frames := []rune(`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`)
	length := len(frames)
	currentFrame := (time.Now().UnixNano() / int64(time.Millisecond) / 150) % int64(length)

	return Info(string(frames[currentFrame]) + " ")
}

func (c *IconRunningComponent) Finalize() {
	atomic.CompareAndSwapUint32(&c.finished, stateRunning, stateDone)
}

func (c *IconRunningComponent) SetFinal(g glint.Component) {
	c.final = g
	c.Finalize()
}
