package command

import (
	"fmt"
	"os"
	"strings"

	getter "github.com/hashicorp/go-getter"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
)

const (
	DefaultEnvironmentsPath = "./environments/"
)

func init() {
	getter.Getters["file"].(*getter.FileGetter).Copy = true
}

func ProvisionCommandFactory(meta Meta) cli.CommandFactory {
	return func() (cli.Command, error) {
		return &Provision{Meta: meta}, nil
	}
}

type Provision struct {
	Meta
}

func (c *Provision) Help() string {
	helpText := `
Usage: nomad-e2e provision <provider> <environment>

  Uses terraform to provision a target test environment to use
  for end-to-end testing.

  The output is a list of environment variables used to configure
  various api clients such as Nomad, Consul and Vault.

General Options:

` + generalOptionsUsage() + `

Provision Options:

  -env-path
    Sets the path for where to search for test environment configuration.
    This defaults to './environments/'.

  -nomad-binary
    Sets the target nomad-binary to use when provisioning a nomad cluster.
	The binary is retrieved by go-getter and can therefore be a local file
	path, remote http url, or other support go-getter uri.

  -destroy
    If set, will destroy the target environment.

  -tf-path
    Sets the path for which terraform state files are stored. Defaults to
	the current working directory.
`
	return strings.TrimSpace(helpText)
}

func (c *Provision) Synopsis() string {
	return "Provisions the target testing environment"
}

func (c *Provision) Run(args []string) int {
	var envPath string
	var nomadBinary string
	var destroy bool
	var tfPath string
	cmdFlags := c.FlagSet("provision")
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&envPath, "env-path", DefaultEnvironmentsPath, "Path to e2e environment terraform configs")
	cmdFlags.StringVar(&nomadBinary, "nomad-binary", "", "")
	cmdFlags.BoolVar(&destroy, "destroy", false, "")
	cmdFlags.StringVar(&tfPath, "tf-path", "", "")

	if err := cmdFlags.Parse(args); err != nil {
		c.logger.Error("failed to parse flags:", "error", err)
		return 1
	}
	if c.verbose {
		c.logger.SetLevel(hclog.Debug)
	}

	args = cmdFlags.Args()
	if len(args) != 2 {
		c.logger.Error("expected 2 args (provider and environment)", "args", args)
		return 0
	}

	env, err := newEnv(envPath, args[0], args[1], tfPath, c.logger)
	if err != nil {
		c.logger.Error("failed to build environment", "error", err)
		return 1
	}

	if destroy {
		if err := env.destroy(); err != nil {
			c.logger.Error("failed to destroy environment", "error", err)
			return 1
		}
		c.logger.Debug("environment successfully destroyed")
		return 0
	}

	// Use go-getter to fetch the nomad binary
	nomadPath, err := fetchBinary(nomadBinary)
	defer os.RemoveAll(nomadPath)
	if err != nil {
		c.logger.Error("failed to fetch nomad binary", "error", err)
		return 1
	}

	results, err := env.provision(nomadPath)
	if err != nil {
		c.logger.Error("", "error", err)
		return 1
	}

	c.Ui.Output(strings.TrimSpace(fmt.Sprintf(`
NOMAD_ADDR=%s
	`, results.nomadAddr)))

	return 0
}
