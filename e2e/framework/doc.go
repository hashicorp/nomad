// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

/*
Package framework implements a model for developing end-to-end test suites. The
model includes a top level Framework which TestSuites can be added to. TestSuites
include conditions under which the suite will run and a list of TestCase
implementations to run. TestCases can be implemented with methods that run
before/after each and all tests.

# Writing Tests

Tests follow a similar patterns as go tests. They are functions that must start
with 'Test' and instead of a *testing.T argument, a *framework.F is passed and
they must have a receiver that implements the TestCase interface.
A crude example as follows:

	// foo_test.go
	type MyTestCase struct {
		framework.TC
	}

	func (tc *MyTestCase) TestMyFoo(f *framework.F) {
		f.T().Log("bar")
	}

	func TestCalledFromGoTest(t *testing.T){
		framework.New().AddSuites(&framework.TestSuite{
			Component:   "foo",
			Cases: []framework.TestCase{
				new(MyTestCase),
			},
		}).Run(t)
	}

Test cases should embed the TC struct which satisfies the TestCase interface.
Optionally a TestCase can also implement the Name() function which returns
a string to name the test case. By default the name is the name of the struct
type, which in the above example would be "MyTestCase"

Test cases may also optionally implement additional interfaces to define setup
and teardown logic:

	BeforeEachTest
	AfterEachTest
	BeforeAllTests
	AfterAllTests

The test case struct allows you to setup and teardown state in the struct that
can be consumed by the tests. For example:

	type ComplexNomadTC struct {
		framework.TC
		jobID string
	}

	func (tc *ComplexNomadTC) BeforeEach(f *framework.F){
		// Do some complex job setup with a unique prefix string
		jobID, err := doSomeComplexSetup(tc.Nomad(), f.ID())
		f.NoError(err)
		f.Set("jobID", jobID)
	}

	func (tc *ComplexNomadTC) TestSomeScenario(f *framework.F){
		jobID := f.Value("jobID").(string)
		doTestThingWithJob(f, tc.Nomad(), jobID)
	}

	func (tc *ComplexNomadTC) TestOtherScenario(f *framework.F){
		jobID := f.Value("jobID").(string)
		doOtherTestThingWithJob(f, tc.Nomad(), jobID)
	}

	func (tc *ComplexNomadTC) AfterEach(f *framework.F){
		jobID := f.Value("jobID").(string)
		_, _, err := tc.Nomad().Jobs().Deregister(jobID, true, nil)
		f.NoError(err)
	}

As demonstrated in the previous example, TC also exposes functions that return
configured api clients including Nomad, Consul and Vault. If Consul or Vault
are not provisioned their respective getter functions will return nil.

# Testify Integration

Test cases expose a T() function to fetch the current *testing.T context.
While this means the author is able to most other testing libraries,
github.com/stretch/testify is recommended and integrated into the framework.
The TC struct also embeds testify assertions that are preconfigured with the
current testing context. Additionally TC comes with a Require() method that
yields a testify Require if that flavor is desired.

	func (tc *MyTestCase) TestWithTestify() {
		err := someErrFunc()
		tc.NoError(err)
		// Or tc.Require().NoError(err)
	}

# Parallelism

The test framework honors go test's parallel feature under certain conditions.
A TestSuite can be created with the Parallel field set to true to enable
parallel execution of the test cases of the suite. Tests within a test case
will be executed sequentially unless f.T().Parallel() is called. Note that if
multiple tests are to be executed in parallel, access to TC is not syncronized.
The *framework.F offers a way to store state between before/after each method if
desired.

	func (tc *MyTestCase) BeforeEach(f *framework.F){
		jobID, _ := doSomeComplexSetup(tc.Nomad(), f.ID())
		f.Set("jobID", jobID)
	}

	func (tc *MyTestCase) TestParallel(f *framework.F){
		f.T().Parallel()
		jobID := f.Value("jobID").(string)
	}

Since test cases have the potential to work with a shared Nomad cluster in parallel
any resources created or destroyed must be prefixed with a unique identifier for
each test case. The framework.F struct exposes an ID() function that will return a
string that is unique with in a test. Therefore, multiple tests with in the case
can reliably create unique IDs between tests and setup/teardown. The string
returned is 8 alpha numeric characters.
*/

// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
package framework
