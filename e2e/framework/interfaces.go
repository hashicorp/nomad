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

// BeforeAllSteps is used to define a method to be called before the execution
// of all test steps.
type BeforeAllSteps interface {
	BeforeAllSteps()
}

// AfterAllSteps is used to define a method to be called after the execution of
// all test steps.
type AfterAllSteps interface {
	AfterAllSteps()
}

// BeforeEachStep is used to define a method to be called before each test step.
type BeforeEachStep interface {
	BeforeEachStep()
}

// AfterEachStep is used to degine a method to be called after each test step.
type AfterEachStep interface {
	AfterEachStep()
}
