package components

import (
	"context"
	"time"

	"github.com/mitchellh/go-glint"
	"github.com/tj/go-spin"
)

// Spinner creates a new spinner. The created spinner should NOT be started
// or data races will occur that can result in a panic.
func Spinner() *SpinnerComponent {
	// Create our spinner and setup our default frames
	s := spin.New()
	s.Set(spin.Default)

	return &SpinnerComponent{
		s: s,
	}
}

type SpinnerComponent struct {
	s    *spin.Spinner
	last time.Time
}

func (c *SpinnerComponent) Body(context.Context) glint.Component {
	current := time.Now()
	if c.last.IsZero() || current.Sub(c.last) > 150*time.Millisecond {
		c.last = current
		c.s.Next()
	}

	return glint.Text(c.s.Current())
}
