package agent

import (
	"context"
	"sync"

	"github.com/mitchellh/go-glint"
	gc "github.com/mitchellh/go-glint/components"
)

// glintStatus implements Status and uses a spinner to show updates.
type glintStatus struct {
	mu      sync.Mutex
	closed  bool
	msg     string
	spinner glint.Component
	text    []glint.Component
}

func newGlintStatus() *glintStatus {
	return &glintStatus{
		spinner: gc.Spinner(),
	}
}

func (s *glintStatus) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = msg
}

func (s *glintStatus) Step(status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine our color
	var style []glint.StyleOption
	switch status {
	case StatusOK:
		style = append(style, glint.Color("lightGreen"))

	case StatusError:
		style = append(style, glint.Color("lightRed"))

	case StatusWarn:
		style = append(style, glint.Color("lightYellow"))
	}

	// If we have a prefix, set that
	if icon, ok := statusIcons[status]; ok {
		msg = icon + " " + msg
	}

	// Clear the message so we don't draw a spinner
	s.msg = ""

	// Add our final message
	s.text = append(s.text, glint.Finalize(glint.Style(
		glint.Text(msg),
		style...,
	)))
}

func (s *glintStatus) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *glintStatus) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.text = nil
	s.msg = ""
}

func (s *glintStatus) Body(context.Context) glint.Component {
	s.mu.Lock()
	defer s.mu.Unlock()

	var cs []glint.Component

	// If we have text we draw that first
	if len(s.text) > 0 {
		cs = append(cs, glint.Finalize(glint.Fragment(s.text...)))
	}

	// If we have a message the spinner is active and we draw that
	if !s.closed && len(s.msg) > 0 {
		cs = append(cs, glint.Layout(
			s.spinner,
			glint.Text(" "),
			glint.Text(s.msg),
		).Row())
	}

	c := glint.Fragment(cs...)
	if s.closed {
		c = glint.Finalize(c)
	}

	return c
}
