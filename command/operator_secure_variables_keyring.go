package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
)

// OperatorSecureVariablesKeyringCommand is a Command implementation
// that handles querying, installing, and removing secure variables
// encryption keys from a keyring.
type OperatorSecureVariablesKeyringCommand struct {
	Meta
}

func (c *OperatorSecureVariablesKeyringCommand) Help() string {
	helpText := `
Usage: nomad operator secure-variables keyring [options]

  Manages encryption keys used for storing secure variables. This command may be
  used to examine active encryption keys in the cluster, rotate keys, add new
  keys from backups, or remove unused keys.

  If ACLs are enabled, this command requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Keyring Options:

  -install=<file>    Install a new encryption key from file. The key file must
                     be a JSON file previously written by Nomad to the keystore.

  -list              List the currently installed keys. This list returns key
                     metadata and not sensitive key material.

  -remove=<key>      Remove the given key from the cluster. This operation may
                     only be performed on keys that are not the active key.

  -rotate            Generate a new encryption key for all future variables.
                     Allows the use of the -full flag.

  -full              When used with -rotate, decrypt all variables and
                     re-encrypt with the new key. The command will immediately
                     return and the re-encryption process will run asynchronously
                     on the leader.

  -verbose           Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSecureVariablesKeyringCommand) Synopsis() string {
	return "Manages secure variables encryption keys"
}

func (c *OperatorSecureVariablesKeyringCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-install": complete.PredictFiles("*.json"),
			"-list":    complete.PredictNothing,
			"-remove":  complete.PredictAnything,
			"-rotate":  complete.PredictNothing,
			"-full":    complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}
func (c *OperatorSecureVariablesKeyringCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSecureVariablesKeyringCommand) Name() string { return "secure-variables keyring" }

func (c *OperatorSecureVariablesKeyringCommand) Run(args []string) int {
	var installKey, removeKey string
	var listKeys, rotateKey, rotateFull, verbose bool

	flags := c.Meta.FlagSet("secure-variables keyring", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&installKey, "install", "", "install key")
	flags.BoolVar(&listKeys, "list", false, "list keys")
	flags.StringVar(&removeKey, "remove", "", "remove key")
	flags.BoolVar(&rotateKey, "rotate", false, "rotate key")

	flags.BoolVar(&rotateFull, "full", false, "full key rotation")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Only accept a single argument
	if rotateKey && listKeys {
		c.Ui.Error("Only a single action is allowed")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	found := listKeys || rotateKey
	for _, arg := range []string{installKey, removeKey} {
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
		c.Ui.Error("One of '-install', '-list', '-remove' or '-rotate' flags must be set")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating nomad cli client: %s", err))
		return 1
	}

	if listKeys {
		resp, _, err := client.Keyring().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.handleKeysResponse(resp, verbose)
		return 0
	}

	if rotateKey {
		resp, _, err := client.Keyring().Rotate(
			&api.KeyringRotateOptions{Full: rotateFull}, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.handleKeysResponse([]*api.RootKeyMeta{resp}, verbose)
		return 0
	}

	if removeKey != "" {
		_, err := client.Keyring().Delete(&api.KeyringDeleteOptions{
			KeyID: removeKey,
		}, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.Ui.Output(fmt.Sprintf("Removed encryption key %s", removeKey))
		return 0
	}

	if installKey != "" {

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
	}

	// Should never make it here
	return 0
}

func (c *OperatorSecureVariablesKeyringCommand) handleKeysResponse(keys []*api.RootKeyMeta, verbose bool) {
	length := fullId
	if !verbose {
		length = 8
	}
	out := make([]string, len(keys)+1)
	out[0] = "Key|Active|Create Time"
	i := 1
	for _, k := range keys {
		out[i] = fmt.Sprintf("%s|%v|%s",
			k.KeyID[:length], k.Active, formatTime(k.CreateTime))
		i = i + 1
	}
	c.Ui.Output(formatList(out))
}
