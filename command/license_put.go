package command

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/pkg/errors"
	"github.com/posener/complete"
)

type LicensePutCommand struct {
	Meta

	testStdin io.Reader
}

func (c *LicensePutCommand) Help() string {
	helpText := `
Usage: nomad license put [options]

  Puts a new license in Servers and Clients

  When ACLs are enabled, this command requires a token with the
  'operator:write' capability.

  Use the -force flag to override the currently installed license with an older
  license.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

License Options:

  -force
	Force is used to override the currently installed license. By default
	Nomad will keep the newest license, as determined by the license issue
	date. Use this flag to apply an older license.

Install a new license from a file:

	$ nomad license put <path>

Install a new license from stdin:

	$ nomad license put -

	`
	return strings.TrimSpace(helpText)
}

func (c *LicensePutCommand) Synopsis() string {
	return "Install a new Nomad Enterprise License"
}

func (c *LicensePutCommand) AutoCompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-force": complete.PredictNothing,
		})
}

func (c *LicensePutCommand) Name() string { return "license put" }

func (c *LicensePutCommand) Run(args []string) int {
	var force bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&force, "force", false, "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	args = flags.Args()
	data, err := c.dataFromArgs(args)
	if err != nil {
		c.Ui.Error(errors.Wrap(err, "Error parsing arguments").Error())
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	opts := &api.ApplyLicenseOptions{
		Force: force,
	}
	_, err = client.Operator().ApplyLicense(data, opts, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error putting license: %v", err))
		return 1
	}

	lic, _, err := client.Operator().LicenseGet(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving new license: %v", err))
		return 1
	}

	if lic.ConfigOutdated {
		c.Ui.Warn(`
WARNING: The server's configured file license is now outdated. Please update or
remove the server's license configuration to prevent initialization issues with
potentially expired licenses.
`) // New line for cli output
	}

	c.Ui.Output("Successfully applied license")
	return 0
}

func (c *LicensePutCommand) dataFromArgs(args []string) (string, error) {
	switch len(args) {
	case 0:
		return "", fmt.Errorf("Missing LICENSE argument")
	case 1:
		return LoadDataSource(args[0], c.testStdin)
	default:
		return "", fmt.Errorf("Too many arguments, exptected 1, got %d", len(args))
	}
}

func loadFromFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Failed to read file: %v", err)
	}
	return string(data), nil
}

func loadFromStdin(testStdin io.Reader) (string, error) {
	var stdin io.Reader = os.Stdin
	if testStdin != nil {
		stdin = testStdin
	}

	var b bytes.Buffer
	if _, err := io.Copy(&b, stdin); err != nil {
		return "", fmt.Errorf("Failed to read stdin: %v", err)
	}
	return b.String(), nil
}

func LoadDataSource(file string, testStdin io.Reader) (string, error) {
	// Handle empty quoted shell parameters
	if len(file) == 0 {
		return "", nil
	}

	if file == "-" {
		if len(file) > 1 {
			return file, nil
		}
		return loadFromStdin(testStdin)
	}

	return loadFromFile(file)
}
