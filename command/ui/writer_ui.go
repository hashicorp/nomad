// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ui

import (
	"errors"
	"fmt"
	"io"

	"github.com/mitchellh/cli"
)

// WriterUI is an implementation of the cli.Ui interface which can be used for
// commands that need to have direct access to the underlying UI readers and
// writers.
type WriterUI struct {
	// Ui is the wrapped cli.Ui that supplies the functions for the thin shims
	Ui cli.Ui

	reader      io.Reader
	writer      io.Writer
	errorWriter io.Writer

	// baseUi stores the basic UI that was used to create this WriterUI. It
	// allows us to call its functions and not implement them again.
	baseUi cli.Ui
}

// NewWriterUI generates a new cli.Ui that can be used for commands that
// need access to the underlying UI's writers for copying large amounts of
// data without local buffering. The caller is required to pass a UI
// chain ending in a cli.BasicUi (or a cli.MockUi for testing).
//
// Currently, the UIs in the chain need to be pointers to a cli.ColoredUi,
// cli.BasicUi, or cli.MockUi to work correctly.
func NewWriterUI(ui cli.Ui) (*WriterUI, error) {
	var done bool
	wUI := WriterUI{Ui: ui}

	for !done {
		if ui == nil {
			break
		}

		switch u := ui.(type) {
		case *cli.MockUi:
			wUI.reader = u.InputReader
			wUI.writer = u.OutputWriter
			wUI.errorWriter = u.ErrorWriter
			wUI.baseUi = u
			done = true
		case *cli.BasicUi:
			wUI.reader = u.Reader
			wUI.writer = u.Writer
			wUI.errorWriter = u.ErrorWriter
			wUI.baseUi = u
			done = true
		case *cli.ColoredUi:
			ui = u.Ui
		default:
			return nil, fmt.Errorf("writer ui: unsupported Ui type: %T", ui)
		}
	}

	if !done {
		return nil, errors.New("failed to generate command UI")
	}

	return &wUI, nil
}

func (w *WriterUI) InputReader() io.Reader  { return w.reader }
func (w *WriterUI) OutputWriter() io.Writer { return w.writer }
func (w *WriterUI) ErrorWriter() io.Writer  { return w.errorWriter }

func (w *WriterUI) Output(message string) { w.Ui.Output(message) }
func (w *WriterUI) Info(message string)   { w.Ui.Info(message) }
func (w *WriterUI) Warn(message string)   { w.Ui.Warn(message) }
func (w *WriterUI) Error(message string)  { w.Ui.Error(message) }

func (w *WriterUI) Ask(query string) (string, error)       { return w.Ui.Ask(query) }
func (w *WriterUI) AskSecret(query string) (string, error) { return w.Ui.AskSecret(query) }
