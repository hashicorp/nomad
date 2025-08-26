// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package clientidentity provides end-to-end tests for Nomad's client identity
// feature. This does not involve running jobs, but instead focuses on the
// identity API to query and force renewals of client identity claims.
//
// In order to run this test suite only, from the e2e directory you can trigger
// go test -v -run '^TestClientIdentity$' ./client_identity
package clientidentity
