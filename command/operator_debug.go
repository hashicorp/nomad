// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-multierror"
	goversion "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/version"
	"github.com/posener/complete"
)

type OperatorDebugCommand struct {
	Meta

	timestamp     string
	collectDir    string
	duration      time.Duration
	interval      time.Duration
	pprofInterval time.Duration
	pprofDuration time.Duration
	logLevel      string
	maxNodes      int
	nodeClass     string
	nodeIDs       []string
	serverIDs     []string
	topics        map[api.Topic][]string
	index         uint64
	consul        *external
	vault         *external
	manifest      []string
	ctx           context.Context
	cancel        context.CancelFunc
	opts          *api.QueryOptions
	verbose       bool
	members       *api.ServerMembers
	nodes         []*api.NodeListStub
}

const (
	userAgent                     = "nomad operator debug"
	clusterDir                    = "cluster"
	clientDir                     = "client"
	serverDir                     = "server"
	intervalDir                   = "interval"
	minimumVersionPprofConstraint = ">= 0.11.0, <= 0.11.2"
)

func (c *OperatorDebugCommand) Help() string {
	helpText := `
Usage: nomad operator debug [options]

  Build an archive containing Nomad cluster configuration and state, and Consul
  and Vault status. Include logs and pprof profiles for selected servers and
  client nodes.

  If ACLs are enabled, this command will require a token with the 'node:read'
  capability to run. In order to collect information, the token will also
  require the 'agent:read' and 'operator:read' capabilities, as well as the
  'list-jobs' capability for all namespaces. To collect pprof profiles the
  token will also require 'agent:write', or enable_debug configuration set to
  true.

  If event stream capture is enabled, the Job, Allocation, Deployment,
  and Evaluation topics require 'namespace:read-job' capabilities, the Node
  topic requires 'node:read'.  A 'management' token is required to capture
  ACLToken, ACLPolicy, or all all events.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Consul Options:

  -consul-http-addr=<addr>
    The address and port of the Consul HTTP agent. Overrides the
    CONSUL_HTTP_ADDR environment variable.

  -consul-token=<token>
    Token used to query Consul. Overrides the CONSUL_HTTP_TOKEN environment
    variable and the Consul token file.

  -consul-token-file=<path>
    Path to the Consul token file. Overrides the CONSUL_HTTP_TOKEN_FILE
    environment variable.

  -consul-client-cert=<path>
    Path to the Consul client cert file. Overrides the CONSUL_CLIENT_CERT
    environment variable.

  -consul-client-key=<path>
    Path to the Consul client key file. Overrides the CONSUL_CLIENT_KEY
    environment variable.

  -consul-ca-cert=<path>
    Path to a CA file to use with Consul. Overrides the CONSUL_CACERT
    environment variable and the Consul CA path.

  -consul-ca-path=<path>
    Path to a directory of PEM encoded CA cert files to verify the Consul
    certificate. Overrides the CONSUL_CAPATH environment variable.

Vault Options:

  -vault-address=<addr>
    The address and port of the Vault HTTP agent. Overrides the VAULT_ADDR
    environment variable.

  -vault-token=<token>
    Token used to query Vault. Overrides the VAULT_TOKEN environment
    variable.

  -vault-client-cert=<path>
    Path to the Vault client cert file. Overrides the VAULT_CLIENT_CERT
    environment variable.

  -vault-client-key=<path>
    Path to the Vault client key file. Overrides the VAULT_CLIENT_KEY
    environment variable.

  -vault-ca-cert=<path>
    Path to a CA file to use with Vault. Overrides the VAULT_CACERT
    environment variable and the Vault CA path.

  -vault-ca-path=<path>
    Path to a directory of PEM encoded CA cert files to verify the Vault
    certificate. Overrides the VAULT_CAPATH environment variable.

Debug Options:

  -duration=<duration>
    Set the duration of the debug capture. Logs will be captured from specified servers and
    nodes at "log-level". Defaults to 2m.

  -event-index=<index>
    Specifies the index to start streaming events from. If the requested index is
    no longer in the buffer the stream will start at the next available index.
    Defaults to 0.

  -event-topic=<Allocation,Evaluation,Job,Node,*>:<filter>
    Enable event stream capture, filtered by comma delimited list of topic filters.
    Examples:
      "all" or "*:*" for all events
      "Evaluation" or "Evaluation:*" for all evaluation events
      "*:example" for all events related to the job "example"
    Defaults to "none" (disabled).

  -interval=<interval>
    The interval between snapshots of the Nomad state. Set interval equal to
    duration to capture a single snapshot. Defaults to 30s.

  -log-level=<level>
    The log level to monitor. Defaults to DEBUG.

  -max-nodes=<count>
    Cap the maximum number of client nodes included in the capture. Defaults
    to 10, set to 0 for unlimited.

  -node-id=<node1>,<node2>
    Comma separated list of Nomad client node ids to monitor for logs, API
    outputs, and pprof profiles. Accepts id prefixes, and "all" to select all
    nodes (up to count = max-nodes). Defaults to "all".

  -node-class=<node-class>
    Filter client nodes based on node class.

  -pprof-duration=<duration>
    Duration for pprof collection. Defaults to 1s or -duration, whichever is less.

  -pprof-interval=<pprof-interval>
    The interval between pprof collections. Set interval equal to
    duration to capture a single snapshot. Defaults to 250ms or
   -pprof-duration, whichever is less.

  -server-id=<server1>,<server2>
    Comma separated list of Nomad server names to monitor for logs, API
    outputs, and pprof profiles. Accepts server names, "leader", or "all".
    Defaults to "all".

  -stale=<true|false>
    If "false", the default, get membership data from the cluster leader. If
    the cluster is in an outage unable to establish leadership, it may be
    necessary to get the configuration from a non-leader server.

  -output=<path>
    Path to the parent directory of the output directory. If specified, no
    archive is built. Defaults to the current directory.

  -verbose
    Enable verbose output.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorDebugCommand) Synopsis() string {
	return "Build a debug archive"
}

func (c *OperatorDebugCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-duration":       complete.PredictAnything,
			"-event-index":    complete.PredictAnything,
			"-event-topic":    complete.PredictAnything,
			"-interval":       complete.PredictAnything,
			"-log-level":      complete.PredictSet("TRACE", "DEBUG", "INFO", "WARN", "ERROR"),
			"-max-nodes":      complete.PredictAnything,
			"-node-class":     NodeClassPredictor(c.Client),
			"-node-id":        NodePredictor(c.Client),
			"-server-id":      ServerPredictor(c.Client),
			"-output":         complete.PredictDirs("*"),
			"-pprof-duration": complete.PredictAnything,
			"-consul-token":   complete.PredictAnything,
			"-vault-token":    complete.PredictAnything,
			"-verbose":        complete.PredictAnything,
		})
}

func (c *OperatorDebugCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// NodePredictor returns a client node predictor
func NodePredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		// note we can't use the -stale flag here because we're in the
		// predictor, but a stale query should be safe for prediction;
		// we also can't use region forwarding because we can't rely
		// on the server being up
		resp, _, err := client.Search().PrefixSearch(
			a.Last, contexts.Nodes, &api.QueryOptions{AllowStale: true})
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Nodes]
	})
}

// NodeClassPredictor returns a client node class predictor
// TODO dmay: Consider API options for node class filtering
func NodeClassPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		// note we can't use the -stale flag here because we're in the
		// predictor, but a stale query should be safe for prediction;
		// we also can't use region forwarding because we can't rely
		// on the server being up
		nodes, _, err := client.Nodes().List(&api.QueryOptions{AllowStale: true})
		if err != nil {
			return []string{}
		}

		// Build map of unique node classes across all nodes
		classes := make(map[string]bool)
		for _, node := range nodes {
			classes[node.NodeClass] = true
		}

		// Iterate over node classes looking for match
		filtered := []string{}
		for class := range classes {
			if strings.HasPrefix(class, a.Last) {
				filtered = append(filtered, class)
			}
		}

		return filtered
	})
}

// ServerPredictor returns a server member predictor
// TODO dmay: Consider API options for server member filtering
func ServerPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		// note we can't use the -stale flag here because we're in the
		// predictor, but a stale query should be safe for prediction;
		// we also can't use region forwarding because we can't rely
		// on the server being up
		members, err := client.Agent().MembersOpts(&api.QueryOptions{AllowStale: true})
		if err != nil {
			return []string{}
		}

		// Iterate over server members looking for match
		filtered := []string{}
		for _, member := range members.Members {
			if strings.HasPrefix(member.Name, a.Last) {
				filtered = append(filtered, member.Name)
			}
		}

		return filtered
	})
}

// queryOpts returns a copy of the shared api.QueryOptions so
// that api package methods can safely modify the options
func (c *OperatorDebugCommand) queryOpts() *api.QueryOptions {
	qo := new(api.QueryOptions)
	*qo = *c.opts
	qo.Params = maps.Clone(c.opts.Params)
	return qo
}

func (c *OperatorDebugCommand) Name() string { return "debug" }

func (c *OperatorDebugCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	var duration, interval, pprofInterval, output, pprofDuration, eventTopic string
	var eventIndex int64
	var nodeIDs, serverIDs string
	var allowStale bool

	flags.StringVar(&duration, "duration", "2m", "")
	flags.Int64Var(&eventIndex, "event-index", 0, "")
	flags.StringVar(&eventTopic, "event-topic", "none", "")
	flags.StringVar(&interval, "interval", "30s", "")
	flags.StringVar(&c.logLevel, "log-level", "DEBUG", "")
	flags.IntVar(&c.maxNodes, "max-nodes", 10, "")
	flags.StringVar(&c.nodeClass, "node-class", "", "")
	flags.StringVar(&nodeIDs, "node-id", "all", "")
	flags.StringVar(&serverIDs, "server-id", "all", "")
	flags.BoolVar(&allowStale, "stale", false, "")
	flags.StringVar(&output, "output", "", "")
	flags.StringVar(&pprofDuration, "pprof-duration", "1s", "")
	flags.StringVar(&pprofInterval, "pprof-interval", "250ms", "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	c.consul = &external{tls: &api.TLSConfig{}}
	flags.StringVar(&c.consul.addrVal, "consul-http-addr", os.Getenv("CONSUL_HTTP_ADDR"), "")
	ssl := os.Getenv("CONSUL_HTTP_SSL")
	c.consul.ssl, _ = strconv.ParseBool(ssl)
	flags.StringVar(&c.consul.auth, "consul-auth", os.Getenv("CONSUL_HTTP_AUTH"), "")
	flags.StringVar(&c.consul.tokenVal, "consul-token", os.Getenv("CONSUL_HTTP_TOKEN"), "")
	flags.StringVar(&c.consul.tokenFile, "consul-token-file", os.Getenv("CONSUL_HTTP_TOKEN_FILE"), "")
	flags.StringVar(&c.consul.tls.ClientCert, "consul-client-cert", os.Getenv("CONSUL_CLIENT_CERT"), "")
	flags.StringVar(&c.consul.tls.ClientKey, "consul-client-key", os.Getenv("CONSUL_CLIENT_KEY"), "")
	flags.StringVar(&c.consul.tls.CACert, "consul-ca-cert", os.Getenv("CONSUL_CACERT"), "")
	flags.StringVar(&c.consul.tls.CAPath, "consul-ca-path", os.Getenv("CONSUL_CAPATH"), "")

	c.vault = &external{tls: &api.TLSConfig{}}
	flags.StringVar(&c.vault.addrVal, "vault-address", os.Getenv("VAULT_ADDR"), "")
	flags.StringVar(&c.vault.tokenVal, "vault-token", os.Getenv("VAULT_TOKEN"), "")
	flags.StringVar(&c.vault.tls.CACert, "vault-ca-cert", os.Getenv("VAULT_CACERT"), "")
	flags.StringVar(&c.vault.tls.CAPath, "vault-ca-path", os.Getenv("VAULT_CAPATH"), "")
	flags.StringVar(&c.vault.tls.ClientCert, "vault-client-cert", os.Getenv("VAULT_CLIENT_CERT"), "")
	flags.StringVar(&c.vault.tls.ClientKey, "vault-client-key", os.Getenv("VAULT_CLIENT_KEY"), "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments: %q", err))
		return 1
	}

	// Parse the capture duration
	d, err := time.ParseDuration(duration)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing duration: %s: %s", duration, err.Error()))
		return 1
	}
	c.duration = d

	// Parse the capture interval
	i, err := time.ParseDuration(interval)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing interval: %s: %s", interval, err.Error()))
		return 1
	}
	c.interval = i

	// Validate interval
	if i.Seconds() > d.Seconds() {
		c.Ui.Error(fmt.Sprintf("Error parsing interval: %s is greater than duration %s", interval, duration))
		return 1
	}

	// Parse and clamp the pprof capture duration
	pd, err := time.ParseDuration(pprofDuration)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing pprof duration: %s: %s", pprofDuration, err.Error()))
		return 1
	}
	if pd.Seconds() > d.Seconds() {
		pd = d
	}
	c.pprofDuration = pd

	// Parse and clamp the pprof capture interval
	pi, err := time.ParseDuration(pprofInterval)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing pprof-interval: %s: %s", pprofInterval, err.Error()))
		return 1
	}
	if pi.Seconds() > pd.Seconds() {
		pi = pd
	}
	c.pprofInterval = pi

	// Parse event stream topic filter
	t, err := topicsFromString(eventTopic)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing event topics: %v", err))
		return 1
	}
	c.topics = t

	// Validate and set initial event stream index
	if eventIndex < 0 {
		c.Ui.Error("Event stream index must be greater than zero")
		return 1
	}
	c.index = uint64(eventIndex)

	// Verify there are no extra arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Initialize capture variables and structs
	c.manifest = make([]string, 0)
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.cancel = cancel
	c.trap()

	// Generate timestamped file name
	format := "2006-01-02-150405Z"
	c.timestamp = time.Now().UTC().Format(format)
	stamped := "nomad-debug-" + c.timestamp

	// Create the output directory
	var tmp string
	if output != "" {
		// User specified output directory
		tmp = filepath.Join(output, stamped)
		_, err := os.Stat(tmp)
		if !os.IsNotExist(err) {
			c.Ui.Error("Output directory already exists")
			return 2
		}
	} else {
		// Generate temp directory
		tmp, err = os.MkdirTemp(os.TempDir(), stamped)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating tmp directory: %s", err.Error()))
			return 2
		}
		defer os.RemoveAll(tmp)
	}

	c.collectDir = tmp

	// Write CLI flags to JSON file
	c.writeFlags(flags)

	// Create an instance of the API client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err.Error()))
		return 1
	}

	c.opts = &api.QueryOptions{
		Region:     c.Meta.region,
		AllowStale: allowStale,
		AuthToken:  c.Meta.token,
	}

	// Get complete list of client nodes
	c.nodes, _, err = client.Nodes().List(c.queryOpts())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node info: %v", err))
		return 1
	}

	// Write nodes to file
	c.reportErr(writeResponseToFile(c.nodes, c.newFile(clusterDir, "nodes.json")))

	// Search all nodes If a node class is specified without a list of node id prefixes
	if c.nodeClass != "" && nodeIDs == "" {
		nodeIDs = "all"
	}

	// Resolve client node id prefixes
	nodesFound := 0
	nodeLookupFailCount := 0
	nodeCaptureCount := 0

	for _, id := range stringToSlice(nodeIDs) {
		if id == "all" {
			// Capture from all nodes using empty prefix filter
			id = ""
		} else {
			// Capture from nodes starting with prefix id
			id = sanitizeUUIDPrefix(id)
		}
		nodes, _, err := client.Nodes().PrefixListOpts(id, c.queryOpts())
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
			return 1
		}

		// Increment fail count if no nodes are found
		if len(nodes) == 0 {
			c.Ui.Error(fmt.Sprintf("No node(s) with prefix %q found", id))
			nodeLookupFailCount++
			continue
		}

		nodesFound += len(nodes)

		// Apply constraints to nodes found
		for _, n := range nodes {
			// Ignore nodes that do not match specified class
			if c.nodeClass != "" && n.NodeClass != c.nodeClass {
				continue
			}

			// Add node to capture list
			c.nodeIDs = append(c.nodeIDs, n.ID)
			nodeCaptureCount++

			// Stop looping when we reach the max
			if c.maxNodes != 0 && nodeCaptureCount >= c.maxNodes {
				break
			}
		}
	}

	// Return error if nodes were specified but none were found
	if len(nodeIDs) > 0 && nodeCaptureCount == 0 {
		if nodeIDs == "all" {
			// It's okay to have zero clients for default "all"
			c.Ui.Info("Note: \"-node-id=all\" specified but no clients found")
		} else {
			c.Ui.Error(fmt.Sprintf("Failed to retrieve clients, 0 nodes found in list: %s", nodeIDs))
			return 1
		}
	}

	// Resolve servers
	c.members, err = client.Agent().MembersOpts(c.queryOpts())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to retrieve server list; err: %v", err))
		return 1
	}

	// Write complete list of server members to file
	c.reportErr(writeResponseToFile(c.members, c.newFile(clusterDir, "members.json")))

	// Get leader and write to file; there's no option for AllowStale
	// on this API and a stale result wouldn't even be meaningful, so
	// only warn if we fail so that we don't stop the rest of the
	// debugging
	leader, err := client.Status().Leader()
	if err != nil {
		c.Ui.Warn(fmt.Sprintf("Failed to retrieve leader; err: %v", err))
	}
	if len(leader) > 0 {
		c.reportErr(writeResponseToFile(leader, c.newFile(clusterDir, "leader.json")))
	}

	// Filter for servers matching criteria
	c.serverIDs, err = filterServerMembers(c.members, serverIDs, c.region)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse server list; err: %v", err))
		return 1
	}

	serversFound := 0
	serverCaptureCount := 0

	if c.members != nil {
		serversFound = len(c.members.Members)
	}
	if c.serverIDs != nil {
		serverCaptureCount = len(c.serverIDs)
	}

	// Return error if servers were specified but not found
	if len(serverIDs) > 0 && serverCaptureCount == 0 {
		c.Ui.Error(fmt.Sprintf("Failed to retrieve servers, 0 members found in list: %s", serverIDs))
		return 1
	}

	// Display general info about the capture
	c.Ui.Output("Starting debugger...")
	c.Ui.Output("")
	c.Ui.Output(fmt.Sprintf("Nomad CLI Version: %s", version.GetVersion().FullVersionNumber(true)))
	c.Ui.Output(fmt.Sprintf("           Region: %s", c.region))
	c.Ui.Output(fmt.Sprintf("        Namespace: %s", c.namespace))
	c.Ui.Output(fmt.Sprintf("          Servers: (%d/%d) %v", serverCaptureCount, serversFound, c.serverIDs))
	c.Ui.Output(fmt.Sprintf("          Clients: (%d/%d) %v", nodeCaptureCount, nodesFound, c.nodeIDs))
	if nodeCaptureCount > 0 && nodeCaptureCount == c.maxNodes {
		c.Ui.Output(fmt.Sprintf("                   Max node count reached (%d)", c.maxNodes))
	}
	if nodeLookupFailCount > 0 {
		c.Ui.Output(fmt.Sprintf("Client fail count: %v", nodeLookupFailCount))
	}
	if c.nodeClass != "" {
		c.Ui.Output(fmt.Sprintf("       Node Class: %s", c.nodeClass))
	}
	c.Ui.Output(fmt.Sprintf("         Interval: %s", interval))
	c.Ui.Output(fmt.Sprintf("         Duration: %s", duration))
	c.Ui.Output(fmt.Sprintf("   pprof Interval: %s", pprofInterval))
	if c.pprofDuration.Seconds() != 1 {
		c.Ui.Output(fmt.Sprintf("   pprof Duration: %s", c.pprofDuration))
	}
	if c.topics != nil {
		c.Ui.Output(fmt.Sprintf("     Event topics: %+v", c.topics))
	}
	c.Ui.Output("")
	c.Ui.Output("Capturing cluster data...")

	// Start collecting data
	err = c.collect(client)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error collecting data: %s", err.Error()))
		return 2
	}

	// Write index json/html manifest files
	c.writeManifest()

	// Exit before archive if output directory was specified
	if output != "" {
		c.Ui.Output(fmt.Sprintf("Created debug directory: %s", c.collectDir))
		return 0
	}

	// Create archive tarball
	archiveFile := stamped + ".tar.gz"
	err = TarCZF(archiveFile, tmp, stamped)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating archive: %s", err.Error()))
		return 2
	}

	// Final output with name of tarball
	c.Ui.Output(fmt.Sprintf("Created debug archive: %s", archiveFile))
	return 0
}

// collect collects data from our endpoints and writes the archive bundle
func (c *OperatorDebugCommand) collect(client *api.Client) error {
	// Start background captures
	c.startMonitors(client)
	c.startEventStream(client)

	// Collect cluster data
	self, err := client.Agent().Self()
	c.reportErr(writeResponseOrErrorToFile(
		self, err, c.newFile(clusterDir, "agent-self.json")))

	namespaces, _, err := client.Namespaces().List(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(
		namespaces, err, c.newFile(clusterDir, "namespaces.json")))

	regions, err := client.Regions().List()
	c.reportErr(writeResponseOrErrorToFile(
		regions, err, c.newFile(clusterDir, "regions.json")))

	// Collect data from Consul
	if c.consul.addrVal == "" {
		c.getConsulAddrFromSelf(self)
	}
	c.collectConsul(clusterDir)

	// Collect data from Vault
	vaultAddr := c.vault.addrVal
	if vaultAddr == "" {
		vaultAddr = c.getVaultAddrFromSelf(self)
	}
	c.collectVault(clusterDir, vaultAddr)

	c.collectAgentHosts(client)
	c.collectPeriodicPprofs(client)

	c.collectPeriodic(client)

	return nil
}

// path returns platform specific paths in the tmp root directory
func (c *OperatorDebugCommand) path(paths ...string) string {
	ps := []string{c.collectDir}
	ps = append(ps, paths...)
	return filepath.Join(ps...)
}

// mkdir creates directories in the tmp root directory
func (c *OperatorDebugCommand) mkdir(paths ...string) error {
	joinedPath := c.path(paths...)

	// Ensure path doesn't escape the sandbox of the capture directory
	escapes := escapingfs.PathEscapesSandbox(c.collectDir, joinedPath)
	if escapes {
		return fmt.Errorf("file path escapes capture directory")
	}

	return escapingfs.EnsurePath(joinedPath, true)
}

// startMonitors starts go routines for each node and client
func (c *OperatorDebugCommand) startMonitors(client *api.Client) {
	for _, id := range c.nodeIDs {
		go c.startMonitor(clientDir, "node_id", id, client)
	}

	for _, id := range c.serverIDs {
		go c.startMonitor(serverDir, "server_id", id, client)
	}
}

// startMonitor starts one monitor api request, writing to a file. It blocks and should be
// called in a go routine. Errors are ignored, we want to build the archive even if a node
// is unavailable
func (c *OperatorDebugCommand) startMonitor(path, idKey, nodeID string, client *api.Client) {
	c.mkdir(path, nodeID)
	fh, err := os.Create(c.path(path, nodeID, "monitor.log"))
	if err != nil {
		return
	}
	defer fh.Close()

	qo := api.QueryOptions{
		Params: map[string]string{
			idKey:       nodeID,
			"log_level": c.logLevel,
		},
		AllowStale: c.queryOpts().AllowStale,
	}

	outCh, errCh := client.Agent().Monitor(c.ctx.Done(), &qo)
	for {
		select {
		case out := <-outCh:
			if out == nil {
				continue
			}
			fh.Write(out.Data)

		case err := <-errCh:
			fh.WriteString(fmt.Sprintf("monitor: %s\n", err.Error()))
			return

		case <-c.ctx.Done():
			return
		}
	}
}

// captureEventStream wraps the event stream capture process.
func (c *OperatorDebugCommand) startEventStream(client *api.Client) {
	c.verboseOut("Launching eventstream goroutine...")

	go func() {
		if err := c.captureEventStream(client); err != nil {
			var es string
			if mErr, ok := err.(*multierror.Error); ok {
				es = multierror.ListFormatFunc(mErr.Errors)
			} else {
				es = err.Error()
			}

			c.Ui.Error(fmt.Sprintf("Error capturing event stream: %s", es))
		}
	}()
}

func (c *OperatorDebugCommand) captureEventStream(client *api.Client) error {
	// Ensure output directory is present
	path := clusterDir
	if err := c.mkdir(c.path(path)); err != nil {
		return err
	}

	// Create the output file
	fh, err := os.Create(c.path(path, "eventstream.json"))
	if err != nil {
		return err
	}
	defer fh.Close()

	// Get handle to events endpoint
	events := client.EventStream()

	// Start streaming events
	eventCh, err := events.Stream(c.ctx, c.topics, c.index, c.queryOpts())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			c.verboseOut("Event stream canceled: No events captured")
			return nil
		}
		return fmt.Errorf("failed to stream events: %w", err)
	}

	eventCount := 0
	errCount := 0
	heartbeatCount := 0
	channelEventCount := 0

	var mErrs *multierror.Error

	for {
		select {
		case event := <-eventCh:
			channelEventCount++
			if event.Err != nil {
				errCount++
				c.verboseOutf("error from event stream: index; %d err: %v", event.Index, event.Err)
				mErrs = multierror.Append(mErrs, fmt.Errorf("error at index: %d, Err: %w", event.Index, event.Err))
				break
			}

			if event.IsHeartbeat() {
				heartbeatCount++
				continue
			}

			for _, e := range event.Events {
				eventCount++
				c.verboseOutf("Event: %4d, Index: %d, Topic: %-10s, Type: %s, FilterKeys: %s", eventCount, e.Index, e.Topic, e.Type, e.FilterKeys)

				bytes, err := json.Marshal(e)
				if err != nil {
					errCount++
					mErrs = multierror.Append(mErrs, fmt.Errorf("failed to marshal json from Topic: %s, Type: %s, Err: %w", e.Topic, e.Type, err))
				}

				n, err := fh.Write(bytes)
				if err != nil {
					errCount++
					mErrs = multierror.Append(mErrs, fmt.Errorf("failed to write bytes to eventstream.json; bytes written: %d, Err: %w", n, err))
					break
				}
				n, err = fh.WriteString("\n")
				if err != nil {
					errCount++
					mErrs = multierror.Append(mErrs, fmt.Errorf("failed to write string to eventstream.json; chars written: %d, Err: %w", n, err))
				}
			}
		case <-c.ctx.Done():
			c.verboseOutf("Event stream captured %d events, %d frames, %d heartbeats, %d errors", eventCount, channelEventCount, heartbeatCount, errCount)
			return mErrs.ErrorOrNil()
		}
	}
}

// collectAgentHosts calls collectAgentHost for each selected node
func (c *OperatorDebugCommand) collectAgentHosts(client *api.Client) {
	for _, n := range c.nodeIDs {
		c.collectAgentHost(clientDir, n, client)
	}

	for _, n := range c.serverIDs {
		c.collectAgentHost(serverDir, n, client)
	}
}

// collectAgentHost gets the agent host data
func (c *OperatorDebugCommand) collectAgentHost(path, id string, client *api.Client) {
	var host *api.HostDataResponse
	var err error
	if path == serverDir {
		host, err = client.Agent().Host(id, "", c.queryOpts())
	} else {
		host, err = client.Agent().Host("", id, c.queryOpts())
	}

	if isRedirectError(err) {
		c.Ui.Warn(fmt.Sprintf("%s/%s: /v1/agent/host unavailable on this agent", path, id))
		return
	}

	if err != nil {
		c.Ui.Error(fmt.Sprintf("%s/%s: Failed to retrieve agent host data, err: %v", path, id, err))

		if strings.Contains(err.Error(), api.PermissionDeniedErrorContent) {
			// Drop a hint to help the operator resolve the error
			c.Ui.Warn("Agent host retrieval requires agent:read ACL or enable_debug=true.  See https://www.nomadproject.io/api-docs/agent#host for more information.")
		}
		return // exit on any error
	}

	path = filepath.Join(path, id)
	c.reportErr(writeResponseToFile(host, c.newFile(path, "agent-host.json")))
}

func (c *OperatorDebugCommand) collectPeriodicPprofs(client *api.Client) {

	pprofNodeIDs := []string{}
	pprofServerIDs := []string{}

	// threadcreate pprof causes a panic on Nomad 0.11.0 to 0.11.2 -- skip those versions
	for _, serverID := range c.serverIDs {
		version := c.getNomadVersion(serverID, "")
		err := checkVersion(version, minimumVersionPprofConstraint)
		if err != nil {
			c.Ui.Warn(fmt.Sprintf("Skipping pprof: %v", err))
		}
		pprofServerIDs = append(pprofServerIDs, serverID)
	}

	for _, nodeID := range c.nodeIDs {
		version := c.getNomadVersion("", nodeID)
		err := checkVersion(version, minimumVersionPprofConstraint)
		if err != nil {
			c.Ui.Warn(fmt.Sprintf("Skipping pprof: %v", err))
		}
		pprofNodeIDs = append(pprofNodeIDs, nodeID)
	}

	// Take the first set of pprofs synchronously...
	c.Ui.Output("    Capture pprofInterval 0000")
	c.collectPprofs(client, pprofServerIDs, pprofNodeIDs, 0)
	if c.pprofInterval == c.pprofDuration {
		return
	}

	// ... and then move the rest off into a goroutine
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, c.duration)
		defer cancel()
		timer, stop := helper.NewSafeTimer(c.pprofInterval)
		defer stop()

		pprofIntervalCount := 1
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				c.Ui.Output(fmt.Sprintf("    Capture pprofInterval %04d", pprofIntervalCount))
				c.collectPprofs(client, pprofServerIDs, pprofNodeIDs, pprofIntervalCount)
				timer.Reset(c.pprofInterval)
				pprofIntervalCount++
			}
		}
	}()
}

// collectPprofs captures the /agent/pprof for each listed node
func (c *OperatorDebugCommand) collectPprofs(client *api.Client, serverIDs, nodeIDs []string, interval int) {
	for _, n := range nodeIDs {
		c.collectPprof(clientDir, n, client, interval)
	}

	for _, n := range serverIDs {
		c.collectPprof(serverDir, n, client, interval)
	}
}

// collectPprof captures pprof data for the node
func (c *OperatorDebugCommand) collectPprof(path, id string, client *api.Client, interval int) {
	pprofDurationSeconds := int(c.pprofDuration.Seconds())
	opts := api.PprofOptions{Seconds: pprofDurationSeconds}
	if path == serverDir {
		opts.ServerID = id
	} else {
		opts.NodeID = id
	}

	path = filepath.Join(path, id)
	filename := fmt.Sprintf("profile_%04d.prof", interval)

	bs, err := client.Agent().CPUProfile(opts, c.queryOpts())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("%s: Failed to retrieve pprof %s, err: %v", filename, path, err))
		if strings.Contains(err.Error(), api.PermissionDeniedErrorContent) {
			// All Profiles require the same permissions, so we only need to see
			// one permission failure before we bail.
			// But lets first drop a hint to help the operator resolve the error

			c.Ui.Warn("Pprof retrieval requires agent:write ACL or enable_debug=true.  See https://www.nomadproject.io/api-docs/agent#agent-runtime-profiles for more information.")
			return // only exit on 403
		}
	} else {
		err := c.writeBytes(path, filename, bs)
		if err != nil {
			c.Ui.Error(err.Error())
		}
	}

	// goroutine debug type 1 = legacy text format for human readable output
	opts.Debug = 1
	c.savePprofProfile(path, "goroutine", opts, client)

	// goroutine debug type 2 = goroutine stacks in panic format
	opts.Debug = 2
	c.savePprofProfile(path, "goroutine", opts, client)

	// Reset to pprof binary format
	opts.Debug = 0

	c.savePprofProfile(path, "goroutine", opts, client)    // Stack traces of all current goroutines
	c.savePprofProfile(path, "trace", opts, client)        // A trace of execution of the current program
	c.savePprofProfile(path, "heap", opts, client)         // A sampling of memory allocations of live objects. You can specify the gc GET parameter to run GC before taking the heap sample.
	c.savePprofProfile(path, "allocs", opts, client)       // A sampling of all past memory allocations
	c.savePprofProfile(path, "threadcreate", opts, client) // Stack traces that led to the creation of new OS threads
}

// savePprofProfile retrieves a pprof profile and writes to disk
func (c *OperatorDebugCommand) savePprofProfile(path string, profile string, opts api.PprofOptions, client *api.Client) {
	fileName := fmt.Sprintf("%s.prof", profile)
	if opts.Debug > 0 {
		fileName = fmt.Sprintf("%s-debug%d.txt", profile, opts.Debug)
	}

	bs, err := retrievePprofProfile(profile, opts, client, c.queryOpts())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("%s: Failed to retrieve pprof %s, err: %s", path, fileName, err.Error()))
	}

	err = c.writeBytes(path, fileName, bs)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("%s: Failed to write file %s, err: %s", path, fileName, err.Error()))
	}
}

// retrievePprofProfile gets a pprof profile from the node specified
// in opts using the API client
func retrievePprofProfile(profile string, opts api.PprofOptions, client *api.Client, qopts *api.QueryOptions) (bs []byte, err error) {
	switch profile {
	case "cpuprofile":
		bs, err = client.Agent().CPUProfile(opts, qopts)
	case "trace":
		bs, err = client.Agent().Trace(opts, qopts)
	default:
		bs, err = client.Agent().Lookup(profile, opts, qopts)
	}

	return bs, err
}

// collectPeriodic runs for duration, capturing the cluster state
// every interval. It flushes and stops the monitor requests
func (c *OperatorDebugCommand) collectPeriodic(client *api.Client) {
	duration := time.After(c.duration)
	// Set interval to 0 so that we immediately execute, wait the interval next time
	interval := time.After(0 * time.Second)
	var intervalCount int
	var name, dir string

	for {
		select {
		case <-duration:
			c.cancel()
			return

		case <-interval:
			name = fmt.Sprintf("%04d", intervalCount)
			dir = filepath.Join(intervalDir, name)
			c.Ui.Output(fmt.Sprintf("    Capture interval %s", name))
			c.collectNomad(dir, client)
			c.collectOperator(dir, client)
			interval = time.After(c.interval)
			intervalCount++

		case <-c.ctx.Done():
			return
		}
	}
}

// collectOperator captures some cluster meta information
func (c *OperatorDebugCommand) collectOperator(dir string, client *api.Client) {
	rc, err := client.Operator().RaftGetConfiguration(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(rc, err, c.newFile(dir, "operator-raft.json")))

	sc, _, err := client.Operator().SchedulerGetConfiguration(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(sc, err, c.newFile(dir, "operator-scheduler.json")))

	ah, _, err := client.Operator().AutopilotServerHealth(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(
		ah, err, c.newFile(dir, "operator-autopilot-health.json")))

	lic, _, err := client.Operator().LicenseGet(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(lic, err, c.newFile(dir, "license.json")))
}

// collectNomad captures the nomad cluster state
func (c *OperatorDebugCommand) collectNomad(dir string, client *api.Client) error {

	js, _, err := client.Jobs().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(js, err, c.newFile(dir, "jobs.json")))

	ds, _, err := client.Deployments().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(ds, err, c.newFile(dir, "deployments.json")))

	es, _, err := client.Evaluations().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(es, err, c.newFile(dir, "evaluations.json")))

	as, _, err := client.Allocations().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(as, err, c.newFile(dir, "allocations.json")))

	ns, _, err := client.Nodes().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(ns, err, c.newFile(dir, "nodes.json")))

	// CSI Plugins - /v1/plugins?type=csi
	ps, _, err := client.CSIPlugins().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(ps, err, c.newFile(dir, "csi-plugins.json")))

	// CSI Plugin details - /v1/plugin/csi/:plugin_id
	for _, p := range ps {
		csiPlugin, _, err := client.CSIPlugins().Info(p.ID, c.queryOpts())
		csiPluginFileName := fmt.Sprintf("csi-plugin-id-%s.json", p.ID)
		c.reportErr(writeResponseOrErrorToFile(csiPlugin, err, c.newFile(dir, csiPluginFileName)))
	}

	// CSI Volumes - /v1/volumes?type=csi
	csiVolumes, _, err := client.CSIVolumes().List(c.queryOpts())
	c.reportErr(writeResponseStreamOrErrorToFile(
		csiVolumes, err, c.newFile(dir, "csi-volumes.json")))

	// CSI Volume details - /v1/volumes/csi/:volume-id
	for _, v := range csiVolumes {
		csiVolume, _, err := client.CSIVolumes().Info(v.ID, c.queryOpts())
		csiFileName := fmt.Sprintf("csi-volume-id-%s.json", v.ID)
		c.reportErr(writeResponseOrErrorToFile(csiVolume, err, c.newFile(dir, csiFileName)))
	}

	metrics, _, err := client.Operator().MetricsSummary(c.queryOpts())
	c.reportErr(writeResponseOrErrorToFile(metrics, err, c.newFile(dir, "metrics.json")))

	return nil
}

// collectConsul calls the Consul API to collect data
func (c *OperatorDebugCommand) collectConsul(dir string) {
	if c.consul.addrVal == "" {
		c.Ui.Output("Consul - Skipping, no API address found")
		return
	}

	c.Ui.Info(fmt.Sprintf("Consul - Collecting Consul API data from: %s", c.consul.addrVal))

	client, err := c.consulAPIClient()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to create Consul API client: %s", err))
		return
	}

	// Exit if we are unable to retrieve the leader
	err = c.collectConsulAPIRequest(client, "/v1/status/leader", dir, "consul-leader.json")
	if err != nil {
		c.Ui.Output(fmt.Sprintf("Unable to contact Consul leader, skipping: %s", err))
		return
	}

	c.collectConsulAPI(client, "/v1/agent/host", dir, "consul-agent-host.json")
	c.collectConsulAPI(client, "/v1/agent/members", dir, "consul-agent-members.json")
	c.collectConsulAPI(client, "/v1/agent/metrics", dir, "consul-agent-metrics.json")
	c.collectConsulAPI(client, "/v1/agent/self", dir, "consul-agent-self.json")
}

func (c *OperatorDebugCommand) consulAPIClient() (*http.Client, error) {
	httpClient := defaultHttpClient()

	err := api.ConfigureTLS(httpClient, c.consul.tls)
	if err != nil {
		return nil, fmt.Errorf("failed to configure TLS: %w", err)
	}

	return httpClient, nil
}

func (c *OperatorDebugCommand) collectConsulAPI(client *http.Client, urlPath string, dir string, file string) {
	err := c.collectConsulAPIRequest(client, urlPath, dir, file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error collecting from Consul API: %s", err.Error()))
	}
}

func (c *OperatorDebugCommand) collectConsulAPIRequest(client *http.Client, urlPath string, dir string, file string) error {
	url := c.consul.addrVal + urlPath

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for Consul API URL=%q: %w", url, err)
	}

	req.Header.Add("X-Consul-Token", c.consul.token())
	req.Header.Add("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	c.writeBody(dir, file, resp, err)

	return nil
}

// collectVault calls the Vault API directly to collect data
func (c *OperatorDebugCommand) collectVault(dir, vault string) error {
	vaultAddr := c.vault.addr(vault)
	if vaultAddr == "" {
		return nil
	}

	c.Ui.Info(fmt.Sprintf("Vault - Collecting Vault API data from: %s", vaultAddr))
	client := defaultHttpClient()
	if c.vault.ssl {
		err := api.ConfigureTLS(client, c.vault.tls)
		if err != nil {
			return fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	req, err := http.NewRequest(http.MethodGet, vaultAddr+"/v1/sys/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for Vault API URL=%q: %w", vaultAddr, err)
	}

	req.Header.Add("X-Vault-Token", c.vault.token())
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	c.writeBody(dir, "vault-sys-health.json", resp, err)

	return nil
}

// writeBytes writes a file to the archive, recording it in the manifest
func (c *OperatorDebugCommand) writeBytes(dir, file string, data []byte) error {
	// Replace invalid characters in filename
	filename := helper.CleanFilename(file, "_")

	relativePath := filepath.Join(dir, filename)
	c.manifest = append(c.manifest, relativePath)
	dirPath := filepath.Join(c.collectDir, dir)
	filePath := filepath.Join(dirPath, filename)

	// Ensure parent directories exist
	err := escapingfs.EnsurePath(dirPath, true)
	if err != nil {
		return fmt.Errorf("failed to create parent directories of %q: %w", dirPath, err)
	}

	// Ensure filename doesn't escape the sandbox of the capture directory
	escapes := escapingfs.PathEscapesSandbox(c.collectDir, filePath)
	if escapes {
		return fmt.Errorf("file path %q escapes capture directory %q", filePath, c.collectDir)
	}

	// Create the file
	fh, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q, err: %w", filePath, err)
	}
	defer fh.Close()

	_, err = fh.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write data to file %q, err: %w", filePath, err)
	}
	return nil
}

// newFilePath returns a validated filepath rooted in the provided directory and
// path. It has been checked that it falls inside the sandbox and has been added
// to the manifest tracking.
func (c *OperatorDebugCommand) newFilePath(dir, file string) (string, error) {

	// Replace invalid characters in filename
	filename := helper.CleanFilename(file, "_")

	relativePath := filepath.Join(dir, filename)
	c.manifest = append(c.manifest, relativePath)
	dirPath := filepath.Join(c.collectDir, dir)
	filePath := filepath.Join(dirPath, filename)

	// Ensure parent directories exist
	err := escapingfs.EnsurePath(dirPath, true)
	if err != nil {
		return "", fmt.Errorf("failed to create parent directories of %q: %w", dirPath, err)
	}

	// Ensure filename doesn't escape the sandbox of the capture directory
	escapes := escapingfs.PathEscapesSandbox(c.collectDir, filePath)
	if escapes {
		return "", fmt.Errorf("file path %q escapes capture directory %q", filePath, c.collectDir)
	}

	return filePath, nil
}

type writerGetter func() (io.WriteCloser, error)

// newFile returns a func that creates a new file for writing and returns it as
// an io.WriterCloser interface. The caller is responsible for closing the
// io.Writer when its done.
//
// Note: methods cannot be generic in go, so this function returns a function
// that closes over our command so that we can still reference the command
// object's fields to validate the file. In future iterations it might be nice
// if we could move most of the command into standalone functions.
func (c *OperatorDebugCommand) newFile(dir, file string) writerGetter {
	return func() (io.WriteCloser, error) {
		filePath, err := c.newFilePath(dir, file)
		if err != nil {
			return nil, err
		}

		writer, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %q: %w", filePath, err)
		}
		return writer, nil
	}
}

// writeResponseToFile writes a response object to a file. It returns an error
// that the caller should report to the UI.
func writeResponseToFile(obj any, getWriterFn writerGetter) error {

	writer, err := getWriterFn()
	if err != nil {
		return err
	}
	defer writer.Close()

	err = writeJSON(obj, writer)
	if err != nil {
		return err
	}
	return nil
}

// writeResponseOrErrorToFile writes a response object to a file, or the error
// for that response if one was received. It returns an error that the caller
// should report to the UI.
func writeResponseOrErrorToFile(obj any, apiErr error, getWriterFn writerGetter) error {

	writer, err := getWriterFn()
	if err != nil {
		return err
	}
	defer writer.Close()

	if apiErr != nil {
		obj = errorWrapper{Error: apiErr.Error()}
	}

	err = writeJSON(obj, writer)
	if err != nil {
		return err
	}
	return nil
}

// writeResponseStreamOrErrorToFile writes a stream of response objects to a
// file in newline-delimited JSON format, or the error for that response if one
// was received. It returns an error that the caller should report to the UI.
func writeResponseStreamOrErrorToFile[T any](obj []T, apiErr error, getWriterFn writerGetter) error {

	writer, err := getWriterFn()
	if err != nil {
		return err
	}
	defer writer.Close()

	if apiErr != nil {
		wrapped := errorWrapper{Error: apiErr.Error()}
		return writeJSON(wrapped, writer)
	}

	err = writeNDJSON(obj, writer)
	if err != nil {
		return err
	}
	return nil
}

// writeNDJSON writes a single Nomad API objects (or response error) to the
// archive file as a JSON object.
func writeJSON(obj any, writer io.Writer) error {
	buf, err := json.Marshal(obj)
	if err != nil {
		buf, err = json.Marshal(errorWrapper{Error: err.Error()})
		if err != nil {
			return fmt.Errorf("could not serialize our own error: %v", err)
		}
	}
	n, err := writer.Write(buf)
	if err != nil {
		return fmt.Errorf("write error, wrote %d bytes of %d: %v", n, len(buf), err)
	}
	return nil
}

// writeNDJSON writes a slice of Nomad API objects to the archive file as
// newline-delimited JSON objects.
func writeNDJSON[T any](data []T, writer io.Writer) error {
	for _, obj := range data {
		err := writeJSON(obj, writer)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		_, err = writer.Write([]byte{'\n'})
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}

// writeError writes a JSON error object to capture errors in the debug bundle without
// reporting
func (c *OperatorDebugCommand) writeError(dir, file string, err error) error {
	bytes, err := json.Marshal(errorWrapper{Error: err.Error()})
	if err != nil {
		return err
	}
	return c.writeBytes(dir, file, bytes)
}

type errorWrapper struct {
	Error string
}

// writeBody is a helper that writes the body of an http.Response to the archive
func (c *OperatorDebugCommand) writeBody(dir, file string, resp *http.Response, err error) {
	if err != nil {
		c.writeError(dir, file, err)
		return
	}

	if resp.ContentLength == 0 {
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.writeError(dir, file, err)
		return
	}

	if err := c.writeBytes(dir, file, body); err != nil {
		c.Ui.Error(err.Error())
	}
}

type flagExport struct {
	Name      string
	Parsed    bool
	Actual    map[string]*flag.Flag
	Formal    map[string]*flag.Flag
	Effective map[string]*flag.Flag // All flags with non-empty value
	Args      []string              // arguments after flags
	OsArgs    []string
}

// writeFlags exports the CLI flags to JSON file
func (c *OperatorDebugCommand) writeFlags(flags *flag.FlagSet) {

	var f flagExport
	f.Name = flags.Name()
	f.Parsed = flags.Parsed()
	f.Formal = make(map[string]*flag.Flag)
	f.Actual = make(map[string]*flag.Flag)
	f.Effective = make(map[string]*flag.Flag)
	f.Args = flags.Args()
	f.OsArgs = os.Args

	// Formal flags (all flags)
	flags.VisitAll(func(flagA *flag.Flag) {
		f.Formal[flagA.Name] = flagA

		// Determine which of thees are "effective" flags by comparing to empty string
		if flagA.Value.String() != "" {
			f.Effective[flagA.Name] = flagA
		}
	})
	// Actual flags (everything passed on cmdline)
	flags.Visit(func(flag *flag.Flag) {
		f.Actual[flag.Name] = flag
	})

	c.reportErr(writeResponseToFile(f, c.newFile(clusterDir, "cli-flags.json")))
}

func (c *OperatorDebugCommand) reportErr(err error) {
	if err != nil {
		c.Ui.Error(err.Error())
	}
}

// writeManifest creates the index files
func (c *OperatorDebugCommand) writeManifest() error {
	// Write the JSON
	path := filepath.Join(c.collectDir, "index.json")
	jsonFh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer jsonFh.Close()

	json.NewEncoder(jsonFh).Encode(c.manifest)

	// Write the HTML
	path = filepath.Join(c.collectDir, "index.html")
	htmlFh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer htmlFh.Close()

	head, _ := template.New("head").Parse("<html><head><title>{{.}}</title></head>\n<body><h1>{{.}}</h1>\n<ul>")
	line, _ := template.New("line").Parse("<li><a href=\"{{.}}\">{{.}}</a></li>\n")
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	tail := "</ul></body></html>\n"

	head.Execute(htmlFh, c.timestamp)
	for _, f := range c.manifest {
		line.Execute(htmlFh, f)
	}
	htmlFh.WriteString(tail)

	return nil
}

// trap captures signals, and closes stopCh
func (c *OperatorDebugCommand) trap() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		<-sigCh
		c.cancel()
	}()
}

func (c *OperatorDebugCommand) verboseOut(out string) {
	if c.verbose {
		c.Ui.Output(out)
	}
}

func (c *OperatorDebugCommand) verboseOutf(format string, a ...interface{}) {
	c.verboseOut(fmt.Sprintf(format, a...))
}

// TarCZF like the tar command, recursively builds a gzip compressed tar
// archive from a directory. If not empty, all files in the bundle are prefixed
// with the target path.
func TarCZF(archive string, src, target string) error {
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	// create the archive
	fh, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer fh.Close()

	zz := gzip.NewWriter(fh)
	defer zz.Close()

	tw := tar.NewWriter(zz)
	defer tw.Close()

	// tar
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		// return on any error
		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// remove leading path to the src, so files are relative to the archive
		path := strings.ReplaceAll(file, src, "")
		if target != "" {
			path = filepath.Join([]string{target, path}...)
		}
		path = strings.TrimPrefix(path, string(filepath.Separator))

		header.Name = path

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// copy the file contents
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		f.Close()

		return nil
	})
}

// filterServerMembers returns a slice of server member names matching the search criteria
func filterServerMembers(serverMembers *api.ServerMembers, serverIDs string, region string) (membersFound []string, err error) {
	if serverMembers.Members == nil {
		return nil, fmt.Errorf("Failed to parse server members, members==nil")
	}

	prefixes := stringToSlice(serverIDs)

	// "leader" is a special case which Nomad handles in the API.  If "leader"
	// appears in serverIDs, add it to membersFound and remove it from the list
	// so that it isn't processed by the range loop
	if slices.Contains(prefixes, "leader") {
		membersFound = append(membersFound, "leader")
		helper.RemoveEqualFold(&prefixes, "leader")
	}

	for _, member := range serverMembers.Members {
		// If region is provided it must match exactly
		if region != "" && member.Tags["region"] != region {
			continue
		}

		// Always include "all"
		if serverIDs == "all" {
			membersFound = append(membersFound, member.Name)
			continue
		}

		// Include member if name matches any prefix from serverIDs
		if helper.StringHasPrefixInSlice(member.Name, prefixes) {
			membersFound = append(membersFound, member.Name)
		}
	}

	return membersFound, nil
}

// stringToSlice splits comma-separated input string into slice, trims
// whitespace, and prunes empty values
func stringToSlice(input string) []string {
	ns := strings.Split(input, ",")
	var out []string
	for _, n := range ns {
		s := strings.TrimSpace(n)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func parseEventTopics(topicList []string) (map[api.Topic][]string, error) {
	topics := make(map[api.Topic][]string)

	var mErrs *multierror.Error

	for _, topic := range topicList {
		k, v, err := parseTopic(topic)
		if err != nil {
			mErrs = multierror.Append(mErrs, err)
		}

		topics[api.Topic(k)] = append(topics[api.Topic(k)], v)
	}

	return topics, mErrs.ErrorOrNil()
}

func parseTopic(input string) (string, string, error) {
	var topic, filter string

	parts := strings.Split(input, ":")
	switch len(parts) {
	case 1:
		// infer wildcard if only given a topic
		topic = input
		filter = "*"
	case 2:
		topic = parts[0]
		filter = parts[1]
	default:
		return "", "", fmt.Errorf("Invalid key value pair for topic: %s", topic)
	}

	return strings.Title(topic), filter, nil
}

func allTopics() map[api.Topic][]string {
	return map[api.Topic][]string{"*": {"*"}}
}

// topicsFromString parses a comma separated list into a topicMap
func topicsFromString(topicList string) (map[api.Topic][]string, error) {
	if topicList == "none" {
		return nil, nil
	}
	if topicList == "all" {
		return allTopics(), nil
	}

	topics := stringToSlice(topicList)
	topicMap, err := parseEventTopics(topics)
	if err != nil {
		return nil, err
	}
	return topicMap, nil
}

// external holds address configuration for Consul and Vault APIs
type external struct {
	tls       *api.TLSConfig
	addrVal   string
	auth      string
	ssl       bool
	tokenVal  string
	tokenFile string
}

func (e *external) addr(defaultAddr string) string {
	if e.addrVal == "" {
		return defaultAddr
	}

	// Return address as-is if it contains a protocol
	if strings.Contains(e.addrVal, "://") {
		return e.addrVal
	}

	if e.ssl {
		return "https://" + e.addrVal
	}

	return "http://" + e.addrVal
}

func (e *external) setAddr(addr string) {
	// Handle no protocol scenario first
	if !strings.Contains(addr, "://") {
		e.addrVal = "http://" + addr
		if e.ssl {
			e.addrVal = "https://" + addr
		}
		return
	}

	// Set SSL boolean based on protocol
	e.ssl = false
	if strings.Contains(addr, "https") {
		e.ssl = true
	}
	e.addrVal = addr
}

func (e *external) token() string {
	if e.tokenVal != "" {
		return e.tokenVal
	}

	if e.tokenFile != "" {
		bs, err := os.ReadFile(e.tokenFile)
		if err == nil {
			return strings.TrimSpace(string(bs))
		}
	}

	return ""
}

func (c *OperatorDebugCommand) getConsulAddrFromSelf(self *api.AgentSelf) string {
	if self == nil {
		return ""
	}

	var consulAddr string
	r, ok := self.Config["Consul"]
	if ok {
		m, ok := r.(map[string]interface{})
		if ok {
			raw := m["EnableSSL"]
			c.consul.ssl, _ = raw.(bool)
			raw = m["Addr"]
			c.consul.setAddr(raw.(string))
			raw = m["Auth"]
			c.consul.auth, _ = raw.(string)
			raw = m["Token"]
			c.consul.tokenVal = raw.(string)

			consulAddr = c.consul.addr("")
		}
	}
	return consulAddr
}

func (c *OperatorDebugCommand) getVaultAddrFromSelf(self *api.AgentSelf) string {
	if self == nil {
		return ""
	}

	var vaultAddr string
	r, ok := self.Config["Vault"]
	if ok {
		m, ok := r.(map[string]interface{})
		if ok {
			raw := m["EnableSSL"]
			c.vault.ssl, _ = raw.(bool)
			raw = m["Addr"]
			c.vault.setAddr(raw.(string))
			raw = m["Auth"]
			c.vault.auth, _ = raw.(string)
			raw = m["Token"]
			c.vault.tokenVal = raw.(string)

			vaultAddr = c.vault.addr("")
		}
	}
	return vaultAddr
}

// defaultHttpClient configures a basic httpClient
func defaultHttpClient() *http.Client {
	httpClient := cleanhttp.DefaultClient()
	transport := httpClient.Transport.(*http.Transport)
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return httpClient
}

// isRedirectError returns true if an error is a redirect error.
func isRedirectError(err error) bool {
	if err == nil {
		return false
	}

	const redirectErr string = `invalid character '<' looking for beginning of value`
	return strings.Contains(err.Error(), redirectErr)
}

// getNomadVersion fetches the version of Nomad running on a given server/client node ID
func (c *OperatorDebugCommand) getNomadVersion(serverID string, nodeID string) string {
	if serverID == "" && nodeID == "" {
		return ""
	}

	version := ""
	if serverID != "" {
		for _, server := range c.members.Members {
			// Raft v2 server
			if server.Name == serverID {
				version = server.Tags["build"]
			}

			// Raft v3 server
			if server.Tags["id"] == serverID {
				version = server.Tags["version"]
			}
		}
	}

	if nodeID != "" {
		for _, node := range c.nodes {
			if node.ID == nodeID {
				version = node.Version
			}
		}
	}

	return version
}

// checkVersion verifies that version satisfies the constraint
func checkVersion(version string, versionConstraint string) error {
	v, err := goversion.NewVersion(version)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	c, err := goversion.NewConstraint(versionConstraint)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	if !c.Check(v) {
		return nil
	}
	return fmt.Errorf("unsupported version=%s matches version filter %s", version, minimumVersionPprofConstraint)
}
