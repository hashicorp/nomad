package command

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	capi "github.com/hashicorp/consul/api"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

func RunCommandFactory() (cli.Command, error) {
	return &Run{}, nil
}

type Run struct {
}

func (c *Run) Help() string {
	helpText := `
Usage: nomad-e2e run
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
	var verbose bool
	cmdFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	cmdFlags.Usage = func() { log.Println(c.Help()) }
	cmdFlags.StringVar(&envPath, "env-path", "./environments/", "Path to e2e environment terraform configs")
	cmdFlags.StringVar(&nomadBinary, "nomad-binary", "", "")
	cmdFlags.StringVar(&tfPath, "tf-path", "", "")
	cmdFlags.StringVar(&run, "run", "", "Regex to target specific test suites/cases")
	cmdFlags.BoolVar(&slow, "slow", false, "Toggle slow running suites")
	cmdFlags.BoolVar(&verbose, "v", false, "Toggle verbose output")

	if err := cmdFlags.Parse(args); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	args = cmdFlags.Args()

	if len(args) == 0 {
		log.Println("No environments specified, running test suite locally...")
		var report *TestReport
		var err error
		if report, err = c.run(&runOpts{
			slow:    slow,
			verbose: verbose,
		}); err != nil {
			log.Fatalf("failed to run test suite: %v", err)
		}
		if report.TotalFailedTests == 0 {
			log.Println("PASSED!")
			if verbose {
				log.Println(report.Summary())
			}
		} else {
			log.Println("***FAILED***")
			log.Println(report.Summary())
		}
		return 0
	}

	environments := []*environment{}
	for _, e := range args {
		if len(strings.Split(e, "/")) != 2 {
			log.Fatalf("argument %s should be formated as <provider>/<environment>", e)
		}
		envs, err := envsFromGlob(envPath, e, tfPath)
		if err != nil {
			log.Fatalf("failed to build environment %s: %v", e, err)
		}
		environments = append(environments, envs...)

	}
	envCount := len(environments)
	// Use go-getter to fetch the nomad binary
	nomadPath, err := fetchBinary(nomadBinary)
	defer os.RemoveAll(nomadPath)
	if err != nil {
		log.Fatal("failed to fetch nomad binary: %v", err)
	}

	log.Printf("Running tests against %d environments...", envCount)
	for i, env := range environments {
		log.Printf("[%d/%d] provisioning %s environment on %s provider", i+1, envCount, env.name, env.provider)
		results, err := env.provision(nomadPath)
		if err != nil {
			log.Fatalf("failed to provision environment %s/%s: %v", env.provider, env.name, err)
		}

		opts := &runOpts{
			provider:   env.provider,
			env:        env.name,
			slow:       slow,
			verbose:    verbose,
			nomadAddr:  results.nomadAddr,
			consulAddr: results.consulAddr,
			vaultAddr:  results.vaultAddr,
		}

		var report *TestReport
		if report, err = c.run(opts); err != nil {
			log.Printf("failed to run tests against environment %s/%s: %v", env.provider, env.name, err)
			return 1
		}
		if report.TotalFailedTests == 0 {
			log.Printf("[%d/%d] %s/%s: PASSED!\n", i, envCount, env.provider, env.name)
			if verbose {
				log.Printf("[%d/%d] %s/%s: %s", i, envCount, env.provider, env.name, report.Summary())
			}
		} else {
			log.Printf("[%d/%d] %s/%s: ***FAILED***\n", i, envCount, env.provider, env.name)
			log.Printf("[%d/%d] %s/%s: %s", i, envCount, env.provider, env.name, report.Summary())
		}
	}
	return 0
}

func (c *Run) run(opts *runOpts) (*TestReport, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(goBin, opts.goArgs()...)
	cmd.Env = opts.goEnv()
	out, err := cmd.StdoutPipe()
	defer out.Close()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	var logger io.Writer
	if opts.verbose {
		logger = os.Stdout
	}

	dec := NewDecoder(out)
	report, err := dec.Decode(logger)
	if err != nil {
		return nil, err
	}

	return report, nil

}

type runOpts struct {
	nomadAddr  string
	consulAddr string
	vaultAddr  string
	provider   string
	env        string
	local      bool
	slow       bool
	verbose    bool
}

func (opts *runOpts) goArgs() []string {
	a := []string{
		"test",
		"-json",
		"github.com/hashicorp/nomad/e2e",
		"-env=" + opts.env,
		"-env.provider=" + opts.provider,
	}

	if opts.slow {
		a = append(a, "-slow")
	}

	if opts.local {
		a = append(a, "-local")
	}
	return a
}

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
