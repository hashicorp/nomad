package command

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorDebugCommand struct {
	Meta

	timestamp  string
	collectDir string
	duration   time.Duration
	interval   time.Duration
	logLevel   string
	stale      bool
	maxNodes   int
	nodeClass  string
	nodeIDs    []string
	serverIDs  []string
	consul     *external
	vault      *external
	manifest   []string
	ctx        context.Context
	cancel     context.CancelFunc
}

const (
	userAgent = "nomad operator debug"
)

func (c *OperatorDebugCommand) Help() string {
	helpText := `
Usage: nomad operator debug [options]

  Build an archive containing Nomad cluster configuration and state, and Consul and Vault
  status. Include logs and pprof profiles for selected servers and client nodes.

  If ACLs are enabled, this command will require a token with the 'node:read'
  capability to run. In order to collect information, the token will also
  require the 'agent:read' and 'operator:read' capabilities, as well as the
  'list-jobs' capability for all namespaces.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Debug Options:

  -duration=<duration>
    The duration of the log monitor command. Defaults to 2m.

  -interval=<interval>
    The interval between snapshots of the Nomad state. If unspecified, only one snapshot is
    captured.

  -log-level=<level>
    The log level to monitor. Defaults to DEBUG.

  -max-nodes=<count>
    Cap the maximum number of client nodes included in the capture.  Defaults to 10, set to 0 for unlimited.

  -node-id=<node>,<node>
    Comma separated list of Nomad client node ids, to monitor for logs and include pprof
    profiles. Accepts id prefixes, and "all" to select all nodes (up to count = max-nodes).

  -node-class=<node-class>
    Filter client nodes based on node class.

  -server-id=<server>,<server>
    Comma separated list of Nomad server names, "leader", or "all" to monitor for logs and include pprof
    profiles.

  -stale=<true|false>
    If "false", the default, get membership data from the cluster leader. If the cluster is in
    an outage unable to establish leadership, it may be necessary to get the configuration from
    a non-leader server.

  -output=<path>
    Path to the parent directory of the output directory. If not specified, an archive is built
    in the current directory.

  -consul-http-addr=<addr>
    The address and port of the Consul HTTP agent. Overrides the CONSUL_HTTP_ADDR environment variable.

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
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorDebugCommand) Synopsis() string {
	return "Build a debug archive"
}

func (c *OperatorDebugCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-duration":     complete.PredictAnything,
			"-interval":     complete.PredictAnything,
			"-log-level":    complete.PredictAnything,
			"-max-nodes":    complete.PredictAnything,
			"-node-class":   complete.PredictAnything,
			"-node-id":      complete.PredictAnything,
			"-server-id":    complete.PredictAnything,
			"-output":       complete.PredictAnything,
			"-consul-token": complete.PredictAnything,
			"-vault-token":  complete.PredictAnything,
		})
}

func (c *OperatorDebugCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorDebugCommand) Name() string { return "debug" }

func (c *OperatorDebugCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	var duration, interval, output string
	var nodeIDs, serverIDs string

	flags.StringVar(&duration, "duration", "2m", "")
	flags.StringVar(&interval, "interval", "2m", "")
	flags.StringVar(&c.logLevel, "log-level", "DEBUG", "")
	flags.IntVar(&c.maxNodes, "max-nodes", 10, "")
	flags.StringVar(&c.nodeClass, "node-class", "", "")
	flags.StringVar(&nodeIDs, "node-id", "", "")
	flags.StringVar(&serverIDs, "server-id", "", "")
	flags.BoolVar(&c.stale, "stale", false, "")
	flags.StringVar(&output, "output", "", "")

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
		tmp, err = ioutil.TempDir(os.TempDir(), stamped)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating tmp directory: %s", err.Error()))
			return 2
		}
		defer os.RemoveAll(tmp)
	}

	c.collectDir = tmp

	// Create an instance of the API client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err.Error()))
		return 1
	}

	// Search all nodes If a node class is specified without a list of node id prefixes
	if c.nodeClass != "" && nodeIDs == "" {
		nodeIDs = "all"
	}

	// Resolve client node id prefixes
	nodesFound := 0
	nodeLookupFailCount := 0
	nodeCaptureCount := 0

	for _, id := range argNodes(nodeIDs) {
		if id == "all" {
			// Capture from all nodes using empty prefix filter
			id = ""
		} else {
			// Capture from nodes starting with prefix id
			id = sanitizeUUIDPrefix(id)
		}
		nodes, _, err := client.Nodes().PrefixList(id)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
			return 1
		}

		// Increment fail count if no nodes are found
		nodesFound = len(nodes)
		if nodesFound == 0 {
			c.Ui.Error(fmt.Sprintf("No node(s) with prefix %q found", id))
			nodeLookupFailCount++
			continue
		}

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
		c.Ui.Error(fmt.Sprintf("Failed to retrieve clients, 0 nodes found in list: %s", nodeIDs))
		return 1
	}

	// Resolve servers
	members, err := client.Agent().Members()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to retrieve server list; err: %v", err))
		return 1
	}
	c.writeJSON("version", "members.json", members, err)
	// We always write the error to the file, but don't range if no members found
	if serverIDs == "all" && members != nil {
		// Special case to capture from all servers
		for _, member := range members.Members {
			c.serverIDs = append(c.serverIDs, member.Name)
		}
	} else {
		for _, id := range argNodes(serverIDs) {
			c.serverIDs = append(c.serverIDs, id)
		}
	}

	serversFound := 0
	serverCaptureCount := 0

	if members != nil {
		serversFound = len(members.Members)
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
	// Version contains cluster meta information
	dir := "version"
	err := c.mkdir(dir)
	if err != nil {
		return err
	}

	self, err := client.Agent().Self()
	c.writeJSON(dir, "agent-self.json", self, err)

	// Fetch data directly from consul and vault. Ignore errors
	var consul, vault string

	if self != nil {
		r, ok := self.Config["Consul"]
		if ok {
			m, ok := r.(map[string]interface{})
			if ok {

				raw := m["Addr"]
				consul, _ = raw.(string)
				raw = m["EnableSSL"]
				ssl, _ := raw.(bool)
				if ssl {
					consul = "https://" + consul
				} else {
					consul = "http://" + consul
				}
			}
		}

		r, ok = self.Config["Vault"]
		if ok {
			m, ok := r.(map[string]interface{})
			if ok {
				raw := m["Addr"]
				vault, _ = raw.(string)
			}
		}
	}

	c.collectConsul(dir, consul)
	c.collectVault(dir, vault)
	c.collectAgentHosts(client)
	c.collectPprofs(client)

	c.startMonitors(client)
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
	return os.MkdirAll(c.path(paths...), 0755)
}

// startMonitors starts go routines for each node and client
func (c *OperatorDebugCommand) startMonitors(client *api.Client) {
	for _, id := range c.nodeIDs {
		go c.startMonitor("client", "node_id", id, client)
	}

	for _, id := range c.serverIDs {
		go c.startMonitor("server", "server_id", id, client)
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
	}

	outCh, errCh := client.Agent().Monitor(c.ctx.Done(), &qo)
	for {
		select {
		case out := <-outCh:
			if out == nil {
				continue
			}
			fh.Write(out.Data)
			fh.WriteString("\n")

		case err := <-errCh:
			fh.WriteString(fmt.Sprintf("monitor: %s\n", err.Error()))
			return

		case <-c.ctx.Done():
			return
		}
	}
}

// collectAgentHosts calls collectAgentHost for each selected node
func (c *OperatorDebugCommand) collectAgentHosts(client *api.Client) {
	for _, n := range c.nodeIDs {
		c.collectAgentHost("client", n, client)
	}

	for _, n := range c.serverIDs {
		c.collectAgentHost("server", n, client)
	}

}

// collectAgentHost gets the agent host data
func (c *OperatorDebugCommand) collectAgentHost(path, id string, client *api.Client) {
	var host *api.HostDataResponse
	var err error
	if path == "server" {
		host, err = client.Agent().Host(id, "", nil)
	} else {
		host, err = client.Agent().Host("", id, nil)
	}

	path = filepath.Join(path, id)
	c.mkdir(path)

	c.writeJSON(path, "agent-host.json", host, err)
}

// collectPprofs captures the /agent/pprof for each listed node
func (c *OperatorDebugCommand) collectPprofs(client *api.Client) {
	for _, n := range c.nodeIDs {
		c.collectPprof("client", n, client)
	}

	for _, n := range c.serverIDs {
		c.collectPprof("server", n, client)
	}

}

// collectPprof captures pprof data for the node
func (c *OperatorDebugCommand) collectPprof(path, id string, client *api.Client) {
	opts := api.PprofOptions{Seconds: 1}
	if path == "server" {
		opts.ServerID = id
	} else {
		opts.NodeID = id
	}

	path = filepath.Join(path, id)
	err := c.mkdir(path)
	if err != nil {
		return
	}

	bs, err := client.Agent().CPUProfile(opts, nil)
	if err == nil {
		c.writeBytes(path, "profile.prof", bs)
	}

	bs, err = client.Agent().Trace(opts, nil)
	if err == nil {
		c.writeBytes(path, "trace.prof", bs)
	}

	bs, err = client.Agent().Lookup("goroutine", opts, nil)
	if err == nil {
		c.writeBytes(path, "goroutine.prof", bs)
	}

	// Gather goroutine text output - debug type 1
	// debug type 1 writes the legacy text format for human readable output
	opts.Debug = 1
	bs, err = client.Agent().Lookup("goroutine", opts, nil)
	if err == nil {
		c.writeBytes(path, "goroutine-debug1.txt", bs)
	}

	// Gather goroutine text output - debug type 2
	// When printing the "goroutine" profile, debug=2 means to print the goroutine
	// stacks in the same form that a Go program uses when dying due to an unrecovered panic.
	opts.Debug = 2
	bs, err = client.Agent().Lookup("goroutine", opts, nil)
	if err == nil {
		c.writeBytes(path, "goroutine-debug2.txt", bs)
	}
}

// collectPeriodic runs for duration, capturing the cluster state every interval. It flushes and stops
// the monitor requests
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
			dir = filepath.Join("nomad", name)
			c.Ui.Output(fmt.Sprintf("    Capture interval %s", name))
			c.collectNomad(dir, client)
			c.collectOperator(dir, client)
			interval = time.After(c.interval)
			intervalCount += 1

		case <-c.ctx.Done():
			return
		}
	}
}

// collectOperator captures some cluster meta information
func (c *OperatorDebugCommand) collectOperator(dir string, client *api.Client) {
	rc, err := client.Operator().RaftGetConfiguration(nil)
	c.writeJSON(dir, "operator-raft.json", rc, err)

	sc, _, err := client.Operator().SchedulerGetConfiguration(nil)
	c.writeJSON(dir, "operator-scheduler.json", sc, err)

	ah, _, err := client.Operator().AutopilotServerHealth(nil)
	c.writeJSON(dir, "operator-autopilot-health.json", ah, err)

	lic, _, err := client.Operator().LicenseGet(nil)
	c.writeJSON(dir, "license.json", lic, err)
}

// collectNomad captures the nomad cluster state
func (c *OperatorDebugCommand) collectNomad(dir string, client *api.Client) error {
	err := c.mkdir(dir)
	if err != nil {
		return err
	}

	var qo *api.QueryOptions

	js, _, err := client.Jobs().List(qo)
	c.writeJSON(dir, "jobs.json", js, err)

	ds, _, err := client.Deployments().List(qo)
	c.writeJSON(dir, "deployments.json", ds, err)

	es, _, err := client.Evaluations().List(qo)
	c.writeJSON(dir, "evaluations.json", es, err)

	as, _, err := client.Allocations().List(qo)
	c.writeJSON(dir, "allocations.json", as, err)

	ns, _, err := client.Nodes().List(qo)
	c.writeJSON(dir, "nodes.json", ns, err)

	ps, _, err := client.CSIPlugins().List(qo)
	c.writeJSON(dir, "plugins.json", ps, err)

	vs, _, err := client.CSIVolumes().List(qo)
	c.writeJSON(dir, "volumes.json", vs, err)

	if metricBytes, err := client.Operator().Metrics(qo); err != nil {
		c.writeError(dir, "metrics.json", err)
	} else {
		c.writeBytes(dir, "metrics.json", metricBytes)
	}

	return nil
}

// collectConsul calls the Consul API directly to collect data
func (c *OperatorDebugCommand) collectConsul(dir, consul string) error {
	addr := c.consul.addr(consul)
	if addr == "" {
		return nil
	}

	client := defaultHttpClient()
	api.ConfigureTLS(client, c.consul.tls)

	req, _ := http.NewRequest("GET", addr+"/v1/agent/self", nil)
	req.Header.Add("X-Consul-Token", c.consul.token())
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	c.writeBody(dir, "consul-agent-self.json", resp, err)

	req, _ = http.NewRequest("GET", addr+"/v1/agent/members", nil)
	req.Header.Add("X-Consul-Token", c.consul.token())
	req.Header.Add("User-Agent", userAgent)
	resp, err = client.Do(req)
	c.writeBody(dir, "consul-agent-members.json", resp, err)

	return nil
}

// collectVault calls the Vault API directly to collect data
func (c *OperatorDebugCommand) collectVault(dir, vault string) error {
	addr := c.vault.addr(vault)
	if addr == "" {
		return nil
	}

	client := defaultHttpClient()
	api.ConfigureTLS(client, c.vault.tls)

	req, _ := http.NewRequest("GET", addr+"/sys/health", nil)
	req.Header.Add("X-Vault-Token", c.vault.token())
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	c.writeBody(dir, "vault-sys-health.json", resp, err)

	return nil
}

// writeBytes writes a file to the archive, recording it in the manifest
func (c *OperatorDebugCommand) writeBytes(dir, file string, data []byte) error {
	relativePath := filepath.Join(dir, file)
	c.manifest = append(c.manifest, relativePath)
	dirPath := filepath.Join(c.collectDir, dir)
	filePath := filepath.Join(dirPath, file)

	// Ensure parent directories exist
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		// Display error immediately -- may not see this if files aren't written
		c.Ui.Error(fmt.Sprintf("failed to create parent directories of \"%s\": %s", dirPath, err.Error()))
		return err
	}

	// Create the file
	fh, err := os.Create(filePath)
	if err != nil {
		// Display error immediately -- may not see this if files aren't written
		c.Ui.Error(fmt.Sprintf("failed to create file \"%s\": %s", filePath, err.Error()))
		return err
	}
	defer fh.Close()

	_, err = fh.Write(data)
	return err
}

// writeJSON writes JSON responses from the Nomad API calls to the archive
func (c *OperatorDebugCommand) writeJSON(dir, file string, data interface{}, err error) error {
	if err != nil {
		return c.writeError(dir, file, err)
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return c.writeError(dir, file, err)
	}
	return c.writeBytes(dir, file, bytes)
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.writeError(dir, file, err)
	}

	c.writeBytes(dir, file, body)
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

// TarCZF, like the tar command, recursively builds a gzip compressed tar archive from a
// directory. If not empty, all files in the bundle are prefixed with the target path
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
		path := strings.Replace(file, src, "", -1)
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

// argNodes splits node ids from the command line by ","
func argNodes(input string) []string {
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

	if !e.ssl {
		if strings.HasPrefix(e.addrVal, "http:") {
			return e.addrVal
		}
		return "http://" + e.addrVal
	}

	if strings.HasPrefix(e.addrVal, "https:") {
		return e.addrVal
	}

	if strings.HasPrefix(e.addrVal, "http:") {
		return "https:" + e.addrVal[5:]
	}

	return "https://" + e.addrVal
}

func (e *external) token() string {
	if e.tokenVal != "" {
		return e.tokenVal
	}

	if e.tokenFile != "" {
		bs, err := ioutil.ReadFile(e.tokenFile)
		if err == nil {
			return strings.TrimSpace(string(bs))
		}
	}

	return ""
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
