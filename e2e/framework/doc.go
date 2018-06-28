/*
Package framework implements a model for developing end-to-end test suites. The
model includes a top level Framework which TestSuites can be added to. TestSuites
include conditions under which the suite will run and a list of TestCase
implementations to run. TestCases can be implemented with methods that run
before/after each and all tests.
*/
package framework
