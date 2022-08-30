package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// OperatorRootKeyringInstallCommand is a Command
// implementation that handles installing variables encryption
// keys from a keyring.
type OperatorRootKeyringInstallCommand struct {
	Meta
}

func (c *OperatorRootKeyringInstallCommand) Help() string {
	helpText := `
Usage: nomad operator root keyring install [options] <filepath>

  Install a new encryption key used for storing variables and workload
  identity signing. The key file must be a JSON file previously written by Nomad
  to the keystore. The key file will be read from stdin by specifying "-",
  otherwise a path to the file is expected.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorRootKeyringInstallCommand) Synopsis() string {
	return "Installs a root encryption key"
}

func (c *OperatorRootKeyringInstallCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
		})
}
func (c *OperatorRootKeyringInstallCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*.json")
}

func (c *OperatorRootKeyringInstallCommand) Name() string {
	return "root keyring install"
}

func (c *OperatorRootKeyringInstallCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("root keyring install", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command requires one argument: <filepath>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	installKey := args[0]

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	if fi, err := os.Stat(installKey); (installKey == "-" || err == nil) && !fi.IsDir() {
		var buf []byte
		if installKey == "-" {
			buf, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
				return 1
			}
		} else {
			buf, err = ioutil.ReadFile(installKey)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
				return 1
			}
		}

		key := &api.RootKey{}
		dec := json.NewDecoder(bytes.NewBuffer(buf))
		if err := dec.Decode(key); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse key file: %v", err))
			return 1
		}

		_, err := client.Keyring().Update(key, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.Ui.Output(fmt.Sprintf("Installed encryption key %s", key.Meta.KeyID))
		return 0
	}

	// Should never make it here
	return 0
}
