// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/helper/escapingfs"
)

// noopReplacer is a noop version of taskenv.TaskEnv.ReplaceEnv.
type noopReplacer struct {
	taskDir string
}

// noopTaskEnv creates a new noopReplacer with the given taskDir.
func noopTaskEnv(taskDir string) interfaces.EnvReplacer {
	return &noopReplacer{taskDir: taskDir}
}

func (*noopReplacer) ReplaceEnv(s string) string {
	return s
}

func (r *noopReplacer) ClientPath(p string, join bool) (string, bool) {
	path, escapes := clientPath(r.taskDir, r.ReplaceEnv(p), join)
	return path, escapes
}

// type upReplacer is a version of taskenv.TaskEnv.ReplaceEnv
// that uppercases all the things.
type upReplacer struct {
	taskDir string
}

// upTaskEnv creates a new noopReplacer with the given taskDir.
func upTaskEnv(taskDir string) interfaces.EnvReplacer {
	return &upReplacer{taskDir: taskDir}
}

func (*upReplacer) ReplaceEnv(s string) string {
	return strings.ToUpper(s)
}

func (r *upReplacer) ClientPath(p string, join bool) (string, bool) {
	path, escapes := clientPath(r.taskDir, r.ReplaceEnv(p), join)
	return path, escapes
}

func clientPath(taskDir, path string, join bool) (string, bool) {
	if !filepath.IsAbs(path) || (escapingfs.PathEscapesSandbox(taskDir, path) && join) {
		path = filepath.Join(taskDir, path)
	}
	path = filepath.Clean(path)
	if taskDir != "" && !escapingfs.PathEscapesSandbox(taskDir, path) {
		return path, false
	}
	return path, true
}
