package framework

import (
	"flag"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var fEnv = flag.String("nomad.env", "", "name of the environment executing against")
var fProvider = flag.String("nomad.env.provider", "", "cloud provider for which environment is executing against")
var fOS = flag.String("nomad.env.os", "", "operating system for which the environment is executing against")
var fArch = flag.String("nomad.env.arch", "", "cpu architecture for which the environment is executing against")
var fTags = flag.String("nomad.env.tags", "", "comma delimited list of tags associated with the environment")
var fLocal = flag.Bool("nomad.local", false, "denotes execution is against a local environment")
var fSlow = flag.Bool("nomad.slow", false, "toggles execution of slow test suites")
var fForceAll = flag.Bool("nomad.force", false, "if set, skips all environment checks when filtering test suites")

var pkgFramework = New()

type Framework struct {
	suites      []*TestSuite
	provisioner Provisioner
	env         Environment

	isLocalRun bool
	slow       bool
	force      bool
}

// Environment includes information about the target environment the test
// framework is targeting. During 'go run', these fields are populated by
// the following flags:
//
//		-nomad.env <string>		"name of the environment executing against"
//		-nomad.env.provider <string>	"cloud provider for which the environment is executing against"
//		-nomad.env.os <string>		"operating system of environment"
//		-nomad.env.arch <string>	"cpu architecture of environment"
//		-nomad.env.tags <string>	"comma delimited list of environment tags"
//
// These flags are not needed when executing locally
type Environment struct {
	Name     string
	Provider string
	OS       string
	Arch     string
	Tags     map[string]struct{}
}

// New creates a Framework
func New() *Framework {
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
		force:       *fForceAll,
	}
}

// AddSuites adds a set of test suites to a Framework
func (f *Framework) AddSuites(s ...*TestSuite) *Framework {
	f.suites = append(f.suites, s...)
	return f
}

// AddSuites adds a set of test suites to the package scoped Framework
func AddSuites(s ...*TestSuite) *Framework {
	pkgFramework.AddSuites(s...)
	return pkgFramework
}

// Run starts the test framework, running each TestSuite
func (f *Framework) Run(t *testing.T) {
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
func Run(t *testing.T) {
	pkgFramework.Run(t)
}

// runSuite is called from Framework.Run inside of a sub test for each TestSuite.
// If skip is returned as true, the test suite is skipped with the error text added
// to the Skip reason
// If skip is false and an error is returned, the test suite is failed.
func (f *Framework) runSuite(t *testing.T, s *TestSuite) (skip bool, err error) {

	// If -nomad.force is set, skip all constraint checks
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

	for _, c := range s.Cases {
		// The test name is set to the name of the implementing type, including package
		name := fmt.Sprintf("%T", c)

		// Each TestCase is provisioned a nomad cluster
		info, err := f.provisioner.ProvisionCluster(ProvisionerOptions{
			Name:         name,
			ExpectConsul: s.Consul,
			ExpectVault:  s.Vault,
		})
		if err != nil {
			return false, fmt.Errorf("could not provision cluster for case: %v", err)
		}
		defer f.provisioner.DestroyCluster(info.ID)
		c.setClusterInfo(info)

		// Each TestCase runs as a subtest of the TestSuite
		t.Run(c.Name(), func(t *testing.T) {
			c.SetT(t)
			// If the TestSuite has Parallel set, all cases run in parallel
			if s.Parallel {
				t.Parallel()
			}

			// Check if the case includes a before all function
			if beforeAllSteps, ok := c.(BeforeAllSteps); ok {
				beforeAllSteps.BeforeAllSteps()
			}

			// Check if the case includes an after all function at the end
			defer func() {
				if afterAllSteps, ok := c.(AfterAllSteps); ok {
					afterAllSteps.AfterAllSteps()
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
				// Each step is run as its own sub test of the case
				// Test cases are never parallel
				t.Run(method.Name, func(t *testing.T) {

					// Since the test function interacts with testing.T through
					// the test case struct, we need to swap the test context for
					// the duration of the step.
					parentT := c.T()
					c.SetT(t)
					if BeforeEachStep, ok := c.(BeforeEachStep); ok {
						BeforeEachStep.BeforeEachStep()
					}
					defer func() {
						if afterEachStep, ok := c.(AfterEachStep); ok {
							afterEachStep.AfterEachStep()
						}
						c.SetT(parentT)
					}()

					//Call the method
					method.Func.Call([]reflect.Value{reflect.ValueOf(c)})
				})
			}
		})

	}

	return false, nil
}

func isTestMethod(m string) bool {
	if !strings.HasPrefix(m, "Test") {
		return false
	}
	// THINKING: adding flag to target a specific step or step regex?
	return true
}
