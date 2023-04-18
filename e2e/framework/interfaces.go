package framework

// TestCase is the interface which an E2E test case implements.
// It is not meant to be implemented directly, instead the struct should embed
// the 'framework.TC' struct
type TestCase interface {
	internalTestCase

	Name() string
}

type internalTestCase interface {
	setClusterInfo(*ClusterInfo)
}

// BeforeAllTests is used to define a method to be called before the execution
// of all tests.
type BeforeAllTests interface {
	BeforeAll(*F)
}

// AfterAllTests is used to define a method to be called after the execution of
// all tests.
type AfterAllTests interface {
	AfterAll(*F)
}

// BeforeEachTest is used to define a method to be called before each test.
type BeforeEachTest interface {
	BeforeEach(*F)
}

// AfterEachTest is used to define a method to be called after each test.
type AfterEachTest interface {
	AfterEach(*F)
}
