// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package subproc provides helper utilities for executing the Nomad binary as
// a child process of the Nomad agent.
//
// The main entrypoint is the Do function, in which the given MainFunc will be
// executed as a sub-process if the first argument matches the subcommand.
//
// Context can be used to create a context.Context object with a given timeout,
// and is expected to be used in conjunction with SetExpiration which uses the
// context's termination to forcefully terminate the child process if it has not
// exited by itself.
package subproc
