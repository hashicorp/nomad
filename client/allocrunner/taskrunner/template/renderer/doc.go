// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package renderer

// This package implements a "hidden" command `nomad template-render`, similarly
// to how we implement logmon, getter, docklog, and executor. This package's
// init() function is evaluated before Nomad's top-level main.go gets a chance
// to parse arguments. This bypasses loading in any behaviors other than the
// small bit of code here.
//
// This command and its subcommands `write` and `read` are only invoked by the
// template runner. See the parent package for the callers.
