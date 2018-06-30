/*
Package framework implements a model for developing end-to-end test suites. The
model includes a top level Framework which TestSuites can be added to. TestSuites
include conditions under which the suite will run and a list of TestCase
implementations to run. TestCases can be implemented with methods that run
before/after each and all tests.

Writing Tests

Tests follow a similar patterns as go tests. They are functions that must start
with 'Test' and instead of a *testing.T argument, they must have a receiver that
implements the TestCase interface. A crude example as follows:

	// foo_test.go
	type MyTestCase struct {
		framework.TC
	}

	func (tc *MyTestCase) TestMyFoo() {
		tc.T().Log("bar")
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
Optionally a TestCase can also override the Name() function of TC which returns
a string to name the test case. By default the name is the name of the struct
type, which in the above example would be "MyTestCase"

Test cases may also optionally implement additional interfaces to define setup
and teardown logic:

	BeforeEachStep
	AfterEachStep
	BeforeAllSteps
	AfterAllSteps

The test case struct allows you to setup and teardown state in the struct that
can be consumed by the tests. For example:

	type ComplexNomadTC struct {
		framework.TC
		jobID string
	}

	func (tc *ComplexNomadTC) BeforeEachStep(){
		// Do some complex job setup with a unique prefix string
		jobID, err := doSomeComplexSetup(tc.Nomad(), tc.Prefix())
		tc.NoError(err)
		tc.jobID = jobID
	}

	func (tc *ComplexNomadTC) TestSomeScenario(){
		doTestThingWithJob(tc.T(), tc.Nomad(), tc.jobID)
	}

	func (tc *ComplexNomadTC) TestOtherScenario(){
		doOtherTestThingWithJob(tc.T(), tc.Nomad(), tc.jobID)
	}

	func (tc *ComplexNomadTC) AfterEachStep(){
		_, _, err := tc.Nomad().Jobs().Deregister(jobID, true, nil)
		tc.Require().NoError(err)
	}

As demonstrated in the previous example, TC also exposes functions that return
configured api clients including Nomad, Consul and Vault. If Consul or Vault
are not provisioned their respective getter functions will return nil.

Testify Integration

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

Parallelism

The test framework honors go test's parallel feature under certain conditions.
A TestSuite can be created with the Parallel field set to true to enable
parallel execution of the test cases of the suite. Tests within a test case
will always be executed sequentially. TC.T() is NOT safe to call from multiple
gorouties, therefore TC.T().Parallel() should NEVER be called from a test of a
TestCase

Since test cases have the potential to work with a shared Nomad cluster in parallel
any resources created or destroyed must be prefixed with a unique identifier for
each test case. The TC struct exposes a Parallel() function that will return a
string that is unique with in a test cases, so multiple tests with in the case
can reliably create unique IDs between tests and setup/teardown. The string
returned is 8 alpha numeric characters with a '-' appended.

*/
package framework
