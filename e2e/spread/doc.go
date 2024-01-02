// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package spread provides end-to-end tests for Nomads spread job specification
// attribute. This does NOT test Nomads spread scheduler.
//
// In order to run this test suite only, from the e2e directory you can trigger
// go test -v -run '^TestSpread' ./spread
package spread
