package components

import (
	"context"
	"time"

	"github.com/mitchellh/go-glint"
)

// Stopwatch creates a new stopwatch component that starts at the given time.
func Stopwatch(start time.Time) *StopwatchComponent {
	return &StopwatchComponent{
		start: start,
	}
}

type StopwatchComponent struct {
	start time.Time
}

func (c *StopwatchComponent) Body(context.Context) glint.Component {
	return glint.Text(time.Now().Sub(c.start).Truncate(100 * time.Millisecond).String())
}
