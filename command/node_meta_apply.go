package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodeMetaApplyCommand struct {
	Meta
}

func (c *NodeMetaApplyCommand) Help() string {
	helpText := `
Usage: nomad node meta apply [-node-id ...] [-unset ...] key1=value1 ... kN=vN

  Modify a node's metadata. This command only works on client agents, and can
  be used to update the scheduling metadata the node registers.

  Changes are batched and may take up to 10 seconds to propagate to the
  servers and affect scheduling.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Node Meta Apply Options:

  -node-id
    Updates metadata on the specified node. If not specified the node receiving
    the request will be used by default.

  -unset key1,...,keyN
    Unset the command separated list of keys.

    Example:
      $ nomad node meta apply -unset testing,tempvar ready=1 role=preinit-db
`
	return strings.TrimSpace(helpText)
}

func (c *NodeMetaApplyCommand) Synopsis() string {
	return "Modify node metadata"
}

func (c *NodeMetaApplyCommand) Name() string { return "node meta apply" }

func (c *NodeMetaApplyCommand) Run(args []string) int {
	var unset, nodeID string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&unset, "unset", "", "")
	flags.StringVar(&nodeID, "node-id", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if unset == "" && len(args) == 0 {
		c.Ui.Error("Must specify -unset or at least 1 key=value pair")
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Lookup nodeID
	if nodeID != "" {
		nodeID, err = lookupNodeID(client.Nodes(), nodeID)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	// Parse parameters
	meta := make(map[string]*string)
	for _, pair := range args {
		kv := strings.SplitN(pair, "=", 2)
		meta[kv[0]] = &kv[1]
	}
	for _, k := range strings.Split(unset, ",") {
		if k != "" {
			meta[k] = nil
		}
	}

	req := api.NodeMetaApplyRequest{
		NodeID: nodeID,
		Meta:   meta,
	}

	if _, err := client.Nodes().Meta().Apply(&req, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying dynamic node metadata: %s", err))
		return 1
	}

	return 0
}

func (c *NodeMetaApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-node-id": complete.PredictNothing,
			"-unset":   complete.PredictNothing,
		})
}

func (c *NodeMetaApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}
