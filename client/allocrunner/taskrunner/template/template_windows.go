// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package template

import (
	"github.com/hashicorp/consul-template/renderer"
)

func RenderFn(taskID, taskDir string, sandboxEnabled bool) func(*renderer.RenderInput) (*renderer.RenderResult, error) {
	return nil
}

func ReaderFn(taskID, taskDir string, sandboxEnabled bool) func(string) ([]byte, error) {
	return nil
}
