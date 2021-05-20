package components

import (
	"context"

	"github.com/cheggaaa/pb/v3"
	"github.com/mitchellh/go-glint"
)

// ProgressElement renders a progress bar. This wraps the cheggaaa/pb package
// since that provides important functionality. This uses single call renders
// to render the progress bar as values change.
type ProgressElement struct {
	*pb.ProgressBar
}

// Progress creates a new progress bar element with the given total.
// For more fine-grained control, please construct a ProgressElement
// directly.
func Progress(total int) *ProgressElement {
	return &ProgressElement{
		ProgressBar: pb.New(total),
	}
}

func (el *ProgressElement) Body(context.Context) glint.Component {
	// If we have no progress bar render nothing.
	if el.ProgressBar == nil {
		return nil
	}

	// Write the current progress
	return glint.TextFunc(func(rows, cols uint) string {
		el.ProgressBar.SetWidth(int(cols))

		return el.ProgressBar.String()
	})
}
