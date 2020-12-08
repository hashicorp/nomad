package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// OperatorKeyringCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type OperatorKeyringCommand struct {
	Meta
}

func (c *OperatorKeyringCommand) Help() string {
	helpText := `
Usage: nomad operator keyring [options]

  Manages encryption keys used for gossip messages between Nomad servers. Gossip
  encryption is optional. When enabled, this command may be used to examine
  active encryption keys in the cluster, add new keys, and remove old ones. When
  combined, this functionality provides the ability to perform key rotation
  cluster-wide, without disrupting the cluster.

  All operations performed by this command can only be run against server nodes.

  All variations of the keyring command return 0 if all nodes reply and there
  are no errors. If any node fails to reply or reports failure, the exit code
  will be 1.

  If ACLs are enabled, this command requires a token with the 'agent:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Keyring Options:

  -install=<key>            Install a new encryption key. This will broadcast
                            the new key to all members in the cluster.
  -list                     List all keys currently in use within the cluster.
  -remove=<key>             Remove the given key from the cluster. This
                            operation may only be performed on keys which are
                            not currently the primary key.
  -use=<key>                Change the primary encryption key, which is used to
                            encrypt messages. The key must already be installed
                            before this operation can succeed.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorKeyringCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}

func (c *OperatorKeyringCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-install": complete.PredictAnything,
			"-list":    complete.PredictNothing,
			"-remove":  complete.PredictAnything,
			"-use":     complete.PredictAnything,
		})
}
func (c *OperatorKeyringCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorKeyringCommand) Name() string { return "operator keyring" }

func (c *OperatorKeyringCommand) Run(args []string) int {
	var installKey, useKey, removeKey string
	var listKeys bool

	flags := c.Meta.FlagSet("operator-keyring", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&installKey, "install", "", "install key")
	flags.StringVar(&useKey, "use", "", "use key")
	flags.StringVar(&removeKey, "remove", "", "remove key")
	flags.BoolVar(&listKeys, "list", false, "list keys")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.Ui,
	}

	// Only accept a single argument
	found := listKeys
	for _, arg := range []string{installKey, useKey, removeKey} {
		if found && len(arg) > 0 {
			c.Ui.Error("Only a single action is allowed")
			c.Ui.Error(commandErrorText(c))
			return 1
		}
		found = found || len(arg) > 0
	}

	// Fail fast if no actionable args were passed
	if !found {
		c.Ui.Error("No actionable argument was passed")
		c.Ui.Error("Either the '-install', '-use', '-remove' or '-list' flag must be set")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// All other operations will require a client connection
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	if listKeys {
		c.Ui.Output("Gathering installed encryption keys...")
		r, err := client.Agent().ListKeys()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.handleKeyResponse(r)
		return 0
	}

	if installKey != "" {
		c.Ui.Output("Installing new gossip encryption key...")
		_, err := client.Agent().InstallKey(installKey)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	if useKey != "" {
		c.Ui.Output("Changing primary gossip encryption key...")
		_, err := client.Agent().UseKey(useKey)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	if removeKey != "" {
		c.Ui.Output("Removing gossip encryption key...")
		_, err := client.Agent().RemoveKey(removeKey)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	// Should never make it here
	return 0
}

func (c *OperatorKeyringCommand) handleKeyResponse(resp *api.KeyringResponse) {
	out := make([]string, len(resp.Keys)+1)
	out[0] = "Key"
	i := 1
	for k := range resp.Keys {
		out[i] = fmt.Sprintf("%s", k)
		i = i + 1
	}
	c.Ui.Output(formatList(out))
}
