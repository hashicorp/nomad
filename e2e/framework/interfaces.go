// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framework

// TestCase is the interface which an E2E test case implements.
// It is not meant to be implemented directly, instead the struct should embed
// the 'framework.TC' struct
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type TestCase interface {
	internalTestCase

	Name() string
}

type internalTestCase interface {
	setClusterInfo(*ClusterInfo)
}

// BeforeAllTests is used to define a method to be called before the execution
// of all tests.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type BeforeAllTests interface {
	BeforeAll(*F)
}

// AfterAllTests is used to define a method to be called after the execution of
// all tests.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type AfterAllTests interface {
	AfterAll(*F)
}

// BeforeEachTest is used to define a method to be called before each test.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type BeforeEachTest interface {
	BeforeEach(*F)
}

// AfterEachTest is used to define a method to be called after each test.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type AfterEachTest interface {
	AfterEach(*F)
}
