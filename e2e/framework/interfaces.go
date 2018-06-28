package framework

import (
	"testing"
)

// Named exists simply to make sure the Name() method was implemented since it
// is the only required method implementation of a test case
type Named interface {
	Name() string
}

type TestCase interface {
	Named
	internalTestCase

	T() *testing.T
	SetT(*testing.T)
}

type internalTestCase interface {
	setClusterInfo(*ClusterInfo)
}

type BeforeAllSteps interface {
	BeforeAllSteps()
}

type AfterAllSteps interface {
	AfterAllSteps()
}

type BeforeEachStep interface {
	BeforeEachStep()
}

type AfterEachStep interface {
	AfterEachStep()
}
