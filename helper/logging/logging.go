// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

// HcLogUI is an implementation of Ui that takes a hclogger
// and uses it to Log the output. It is intended for write only
// use cases and the Ask/AskSecret methods are not implemented.
type HcLogUI struct {
	Log hclog.Logger
}

func (l *HcLogUI) Ask(query string) (string, error) {
	return "", fmt.Errorf("Ask is not supported in this implementation")
}

func (l *HcLogUI) AskSecret(query string) (string, error) {
	return "", fmt.Errorf("AskSecret is not supported in this implementation")
}

func (l *HcLogUI) Output(message string) {
	l.Log.Info(message)
}

func (l *HcLogUI) Info(message string) {
	l.Log.Info(message)
}

func (l *HcLogUI) Error(message string) {
	l.Log.Error(message)
}

func (l *HcLogUI) Warn(message string) {
	l.Log.Warn(message)
}
