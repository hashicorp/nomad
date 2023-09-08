// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framework

import (
	"flag"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

const frameworkHelp = `
Usage: go test -v ./e2e [options]

These flags are coarse overrides for the test environment.

  -forceRun    skip all environment checks when filtering test suites
  -local       denotes execution is against a local environment
  -slow        include execution of slow test suites
  -suite       run specified test suite
  -showHelp    shows this help text

TestSuites can request Constraints on the Framework.Environment so that tests
are only run in the appropriate conditions. These environment flags provide
the information for those constraints.

  -env=string           name of the environment
  -env.arch=string      cpu architecture of the targets
  -env.os=string        operating system of the targets
  -env.provider=string  cloud provider of the environment
  -env.tags=string      comma delimited list of tags for the environment

`

var fHelp = flag.Bool("showHelp", false, "print the help screen")
var fLocal = flag.Bool("local", false,
	"denotes execution is against a local environment")
var fSlow = flag.Bool("slow", false, "toggles execution of slow test suites")
var fForceRun = flag.Bool("forceRun", false,
	"if set, skips all environment checks when filtering test suites")
var fSuite = flag.String("suite", "", "run specified test suite")

// Environment flags
// TODO:
// It would be nice if we could match the target environment against
// the tests automatically so that we always avoid running tests that
// don't apply, and then have these flags override that behavior.
var fEnv = flag.String("env", "", "name of the environment executing against")
var fProvider = flag.String("env.provider", "",
	"cloud provider for which environment is executing against")
var fOS = flag.String("env.os", "",
	"operating system for which the environment is executing against")
var fArch = flag.String("env.arch", "",
	"cpu architecture for which the environment is executing against")
var fTags = flag.String("env.tags", "",
	"comma delimited list of tags associated with the environment")

// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type Framework struct {
	suites      []*TestSuite
	provisioner Provisioner
	env         Environment

	isLocalRun bool
	slow       bool
	force      bool
	suite      string
}

// Environment contains information about the test target environment, used
// to constrain the set of tests run. See the environment flags above.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type Environment struct {
	Name     string
	Provider string
	OS       string
	Arch     string
	Tags     map[string]struct{}
}

// New creates a Framework
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func New() *Framework {
	flag.Parse()
	if *fHelp {
		log.Fatal(frameworkHelp)
	}
	env := Environment{
		Name:     *fEnv,
		Provider: *fProvider,
		OS:       *fOS,
		Arch:     *fArch,
		Tags:     map[string]struct{}{},
	}
	for _, tag := range strings.Split(*fTags, ",") {
		env.Tags[tag] = struct{}{}
	}
	return &Framework{
		provisioner: DefaultProvisioner,
		env:         env,
		isLocalRun:  *fLocal,
		slow:        *fSlow,
		force:       *fForceRun,
		suite:       *fSuite,
	}
}

// AddSuites adds a set of test suites to a Framework
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func (f *Framework) AddSuites(s ...*TestSuite) *Framework {
	f.suites = append(f.suites, s...)
	return f
}

var pkgSuites []*TestSuite

// AddSuites adds a set of test suites to the package scoped Framework
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func AddSuites(s ...*TestSuite) {
	pkgSuites = append(pkgSuites, s...)
}

// Run starts the test framework, running each TestSuite
func (f *Framework) Run(t *testing.T) {
	info, err := f.provisioner.SetupTestRun(t, SetupOptions{})
	if err != nil {
		require.NoError(t, err, "could not provision cluster")
	}
	defer f.provisioner.TearDownTestRun(t, info.ID)

	for _, s := range f.suites {
		t.Run(s.Component, func(t *testing.T) {
			skip, err := f.runSuite(t, s)
			if skip {
				t.Skipf("skipping suite '%s': %v", s.Component, err)
				return
			}
			if err != nil {
				t.Errorf("error starting suite '%s': %v", s.Component, err)
			}
		})
	}
}

// Run starts the package scoped Framework, running each TestSuite
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func Run(t *testing.T) {
	f := New()
	f.AddSuites(pkgSuites...)
	f.Run(t)
}

// runSuite is called from Framework.Run inside of a sub test for each TestSuite.
// If skip is returned as true, the test suite is skipped with the error text added
// to the Skip reason
// If skip is false and an error is returned, the test suite is failed.
func (f *Framework) runSuite(t *testing.T, s *TestSuite) (skip bool, err error) {

	// If -forceRun is set, skip all constraint checks
	if !f.force {
		// If this is a local run, check that the suite supports running locally
		if !s.CanRunLocal && f.isLocalRun {
			return true, fmt.Errorf("local run detected and suite cannot run locally")
		}

		// Check that constraints are met
		if err := s.Constraints.matches(f.env); err != nil {
			return true, fmt.Errorf("constraint failed: %v", err)
		}

		// Check the slow toggle and if the suite's slow flag is that same
		if f.slow != s.Slow {
			return true, fmt.Errorf("framework slow suite configuration is %v but suite is %v", f.slow, s.Slow)
		}
	}

	// If -suite is set, skip any suite that is not the one specified.
	if f.suite != "" && f.suite != s.Component {
		return true, fmt.Errorf("only running suite %q", f.suite)
	}

	info, err := f.provisioner.SetupTestSuite(t, SetupOptions{
		Name:         s.Component,
		ExpectConsul: s.Consul,
		ExpectVault:  s.Vault,
	})
	require.NoError(t, err, "could not provision cluster")
	defer f.provisioner.TearDownTestSuite(t, info.ID)

	for _, c := range s.Cases {
		f.runCase(t, s, c)
	}

	return false, nil
}

func (f *Framework) runCase(t *testing.T, s *TestSuite, c TestCase) {

	// The test name is set to the name of the implementing type, including package
	name := fmt.Sprintf("%T", c)

	// The ClusterInfo handle should be used by each TestCase to isolate
	// job/task state created during the test.
	info, err := f.provisioner.SetupTestCase(t, SetupOptions{
		Name:         name,
		ExpectConsul: s.Consul,
		ExpectVault:  s.Vault,
	})
	if err != nil {
		t.Errorf("could not provision cluster for case: %v", err)
	}
	defer f.provisioner.TearDownTestCase(t, info.ID)
	c.setClusterInfo(info)

	// Each TestCase runs as a subtest of the TestSuite
	t.Run(c.Name(), func(t *testing.T) {
		// If the TestSuite has Parallel set, all cases run in parallel
		if s.Parallel {
			ci.Parallel(t)
		}

		f := newF(t)

		// Check if the case includes a before all function
		if beforeAllTests, ok := c.(BeforeAllTests); ok {
			beforeAllTests.BeforeAll(f)
		}

		// Check if the case includes an after all function at the end
		defer func() {
			if afterAllTests, ok := c.(AfterAllTests); ok {
				afterAllTests.AfterAll(f)
			}
		}()

		// Here we need to iterate through the methods of the case to find
		// ones that are test functions
		reflectC := reflect.TypeOf(c)
		for i := 0; i < reflectC.NumMethod(); i++ {
			method := reflectC.Method(i)
			if ok := isTestMethod(method.Name); !ok {
				continue
			}
			// Each test is run as its own sub test of the case
			// Test cases are never parallel
			t.Run(method.Name, func(t *testing.T) {

				cF := newFFromParent(f, t)
				if BeforeEachTest, ok := c.(BeforeEachTest); ok {
					BeforeEachTest.BeforeEach(cF)
				}
				defer func() {
					if afterEachTest, ok := c.(AfterEachTest); ok {
						afterEachTest.AfterEach(cF)
					}
				}()

				//Call the method
				method.Func.Call([]reflect.Value{reflect.ValueOf(c), reflect.ValueOf(cF)})
			})
		}
	})
}

func isTestMethod(m string) bool {
	return strings.HasPrefix(m, "Test")
}
