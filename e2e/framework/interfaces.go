package framework

import (
	"testing"
)

// TestCase is the interface which an E2E test case implements.
// It is not meant to be implemented directly, instead the struct should embed
// the 'framework.TC' struct
type TestCase interface {
	internalTestCase

	Name() string
	T() *testing.T
	SetT(*testing.T)
}

type internalTestCase interface {
	setClusterInfo(*ClusterInfo)
}

// BeforeAllTests is used to define a method to be called before the execution
// of all tests.
type BeforeAllTests interface {
	BeforeAll()
}

// AfterAllTests is used to define a method to be called after the execution of
// all tests.
type AfterAllTests interface {
	AfterAll()
}

// BeforeEachTest is used to define a method to be called before each test.
type BeforeEachTest interface {
	BeforeEach()
}

// AfterEachTest is used to degine a method to be called after each test.
type AfterEachTest interface {
	AfterEach()
}
