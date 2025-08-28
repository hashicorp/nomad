// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package clientintro provides end-to-end tests for Nomad's client introduction
// feature. This does not involve running jobs and does not run against the
// nightly cluster. Instead it uses local agents to verify client introduction
// behavior.
//
// In order to run this test suite only, from the e2e directory you can trigger
// 'go test -v -run '^TestClientIntro$' ./client_intro' or from the top
// level you can use the 'integration-test-client-intro' make target.
package clientintro
