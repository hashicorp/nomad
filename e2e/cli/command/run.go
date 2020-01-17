package command

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	capi "github.com/hashicorp/consul/api"
	hclog "github.com/hashicorp/go-hclog"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

func RunCommandFactory(meta Meta) cli.CommandFactory {
	return func() (cli.Command, error) {
		return &Run{Meta: meta}, nil
	}
}

type Run struct {
	Meta
}

func (c *Run) Help() string {
	helpText := `
Usage: nomad-e2e run (<provider>/<name>)...

  Two modes exist when using the run command.

  When no arguments are given to the run command, it will launch
  the e2e test suite against the Nomad cluster specified by the
  NOMAD_ADDR environment variable. If this is not set, it defaults
  to 'http://localhost:4646'

  Multiple arguments may be given to specify one or more environments to
  provision and run the e2e tests against. These are given in the form of
  <provider>/<name>. Globs are support, for example 'aws/*' would run tests
  against all of the environments under the aws provider. When using this mode,
  all of the provision flags are supported.

General Options:

` + generalOptionsUsage() + `

Run Options:

  -run regex
    Sets a regular expression for what tests to run. Uses '/' as a separator
	to allow hierarchy between Suite/Case/Test.

	Example '-run MyTestSuite' would only run tests under the MyTestSuite suite.

  -slow
    If set, will only run test suites marked as slow.
`
	return strings.TrimSpace(helpText)
}

func (c *Run) Synopsis() string {
	return "Runs the e2e test suite"
}

func (c *Run) Run(args []string) int {
	var envPath string
	var nomadBinary string
	var tfPath string
	var slow bool
	var run string
	cmdFlags := c.FlagSet("run")
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&envPath, "env-path", DefaultEnvironmentsPath, "Path to e2e environment terraform configs")
	cmdFlags.StringVar(&nomadBinary, "nomad-binary", "", "")
	cmdFlags.StringVar(&tfPath, "tf-path", "", "")
	cmdFlags.StringVar(&run, "run", "", "Regex to target specific test suites/cases")
	cmdFlags.BoolVar(&slow, "slow", false, "Toggle slow running suites")

	if err := cmdFlags.Parse(args); err != nil {
		c.logger.Error("failed to parse flags", "error", err)
		return 1
	}
	if c.verbose {
		c.logger.SetLevel(hclog.Debug)
	}

	args = cmdFlags.Args()

	if len(args) == 0 {
		c.logger.Info("no environments specified, running test suite locally")
		report, err := c.runTest(&runOpts{
			run:     run,
			slow:    slow,
			verbose: c.verbose,
		})
		if err != nil {
			c.logger.Error("failed to run test suite", "error", err)
			return 1
		}
		if report.TotalFailedTests > 0 {
			c.Ui.Error("***FAILED***")
			c.Ui.Error(report.Summary())
			return 1
		}
		c.Ui.Output("PASSED!")
		if c.verbose {
			c.Ui.Output(report.Summary())
		}
		return 0
	}

	environments := []*environment{}
	for _, e := range args {
		if len(strings.Split(e, "/")) != 2 {
			c.logger.Error("argument should be formated as <provider>/<environment>", "args", e)
			return 1
		}
		envs, err := envsFromGlob(envPath, e, tfPath, c.logger)
		if err != nil {
			c.logger.Error("failed to build environment", "environment", e, "error", err)
			return 1
		}
		environments = append(environments, envs...)

	}

	// Use go-getter to fetch the nomad binary
	nomadPath, err := fetchBinary(nomadBinary)
	defer os.RemoveAll(nomadPath)
	if err != nil {
		c.logger.Error("failed to fetch nomad binary", "error", err)
		return 1
	}

	envCount := len(environments)
	c.logger.Debug("starting tests", "totalEnvironments", envCount)
	failedEnvs := map[string]*TestReport{}
	for i, env := range environments {
		logger := c.logger.With("name", env.name, "provider", env.provider)
		logger.Debug("provisioning environment")
		results, err := env.provision(nomadPath)
		if err != nil {
			logger.Error("failed to provision environment", "error", err)
			return 1
		}

		opts := &runOpts{
			provider:   env.provider,
			env:        env.name,
			slow:       slow,
			run:        run,
			verbose:    c.verbose,
			nomadAddr:  results.nomadAddr,
			consulAddr: results.consulAddr,
			vaultAddr:  results.vaultAddr,
		}

		var report *TestReport
		if report, err = c.runTest(opts); err != nil {
			logger.Error("failed to run tests against environment", "error", err)
			return 1
		}
		if report.TotalFailedTests > 0 {
			c.Ui.Error(fmt.Sprintf("[%d/%d] %s: ***FAILED***", i+1, envCount, env.canonicalName()))
			c.Ui.Error(fmt.Sprintf("[%d/%d] %s: %s", i+1, envCount, env.canonicalName(), report.Summary()))
			failedEnvs[env.canonicalName()] = report
		}

		c.Ui.Output(fmt.Sprintf("[%d/%d] %s: PASSED!", i+1, envCount, env.canonicalName()))
		if c.verbose {
			c.Ui.Output(fmt.Sprintf("[%d/%d] %s: %s", i+1, envCount, env.canonicalName(), report.Summary()))
		}
	}

	if len(failedEnvs) > 0 {
		c.Ui.Error(fmt.Sprintf("The following environments ***FAILED***"))
		for name, report := range failedEnvs {
			c.Ui.Error(fmt.Sprintf("  [%s]: %d out of %d suite failures",
				name, report.TotalFailedSuites, report.TotalSuites))
		}
		return 1
	}
	c.Ui.Output("All Environments PASSED!")
	return 0
}

func (c *Run) runTest(opts *runOpts) (*TestReport, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(goBin, opts.goArgs()...)
	cmd.Env = opts.goEnv()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		// should command fail, log here then proceed to generate test report
		// to report more informative info about which tests fail
		c.logger.Error("test command failed", "error", err)
	}

	dec := NewDecoder(out)
	report, err := dec.Decode(c.logger.Named("run.gotest"))
	if err != nil {
		return nil, err
	}

	return report, nil

}

// runOpts contains fields used to build the arguments and environment variabled
// nessicary to run go test and initialize the e2e framework
type runOpts struct {
	nomadAddr  string
	consulAddr string
	vaultAddr  string
	provider   string
	env        string
	run        string
	local      bool
	slow       bool
	verbose    bool
}

// goArgs returns the list of arguments passed to the go command to start the
// e2e test framework
func (opts *runOpts) goArgs() []string {
	a := []string{
		"test",
		"-json",
	}

	if opts.run != "" {
		a = append(a, "-run=TestE2E/"+opts.run)
	}

	a = append(a, []string{
		"github.com/hashicorp/nomad/e2e",
		"-env=" + opts.env,
		"-env.provider=" + opts.provider,
	}...)

	if opts.slow {
		a = append(a, "-slow")
	}

	if opts.local {
		a = append(a, "-local")
	}
	return a
}

// goEnv returns the list of environment variabled passed to the go command to start
// the e2e test framework
func (opts *runOpts) goEnv() []string {
	env := append(os.Environ(), "NOMAD_E2E=1")
	if opts.nomadAddr != "" {
		env = append(env, "NOMAD_ADDR="+opts.nomadAddr)
	}
	if opts.consulAddr != "" {
		env = append(env, fmt.Sprintf("%s=%s", capi.HTTPAddrEnvName, opts.consulAddr))
	}
	if opts.vaultAddr != "" {
		env = append(env, fmt.Sprintf("%s=%s", vapi.EnvVaultAddress, opts.consulAddr))
	}

	return env
}
