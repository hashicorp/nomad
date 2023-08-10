// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package servicediscovery provides end-to-end tests for Nomads service
// discovery feature. It tests all supported discovery providers and ensures
// Nomad can handle operator changes to services with the desired effects.
//
// Subsystems of service discovery such as Consul Connect or Consul Template
// have their own suite of tests.
//
// In order to run this test suite only, from the e2e directory you can trigger
// go test -v -run '^TestServiceDiscovery$' ./servicediscovery
package servicediscovery
