package command

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type DebugCommand struct {
	Meta

	timestamp   string
	collectDir  string
	duration    time.Duration
	interval    time.Duration
	logLevel    string
	nodeIDs     []string
	serverIDs   []string
	consulToken string
	vaultToken  string
	manifest    []string
	ctx         context.Context
	cancel      context.CancelFunc
}

const (
	userAgent = "nomad debug"
)

func (c *DebugCommand) Help() string {
	helpText := `
Usage: nomad debug [options]

  Build an archive containing Nomad cluster configuration and state, and Consul and Vault
  status. Include logs and pprof profiles for selected servers and client nodes.

General Options:

  ` + generalOptionsUsage() + `

Debug Options:

  -duration=2m
   The duration of the log monitor command. Defaults to 2m.

  -interval=2m
   The interval between snapshots of the Nomad state. If unspecified, only one snapshot is
   captured.

  -log-level=DEBUG
   The log level to monitor. Defaults to DEBUG.

  -node-id=n1,n2
   Comma separated list of Nomad client node ids, to monitor for logs and include pprof
   profiles. Accepts id prefixes.

  -server-id=s1,s2
   Comma separated list of Nomad server names, or "leader" to monitor for logs and include pprof
   profiles.

  -output=path
   Path to the parent directory of the output directory. If not specified, an archive is built
   in the current directory.

  -consul-token
   Token used to query Consul. Defaults to CONSUL_HTTP_TOKEN or the contents of
   CONSUL_HTTP_TOKEN_FILE

  -vault-token
   Token used to query Vault. Defaults to VAULT_TOKEN
`
	return strings.TrimSpace(helpText)
}

func (c *DebugCommand) Synopsis() string {
	return "Build a debug archive"
}

func (c *DebugCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-duration":     complete.PredictAnything,
			"-interval":     complete.PredictAnything,
			"-log-level":    complete.PredictAnything,
			"-node-id":      complete.PredictAnything,
			"-server-id":    complete.PredictAnything,
			"-output":       complete.PredictAnything,
			"-consul-token": complete.PredictAnything,
			"-vault-token":  complete.PredictAnything,
		})
}

func (c *DebugCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *DebugCommand) Name() string { return "debug" }

func (c *DebugCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	var duration, interval, output string
	var nodeIDs, serverIDs string

	flags.StringVar(&duration, "duration", "2m", "")
	flags.StringVar(&interval, "interval", "2m", "")
	flags.StringVar(&c.logLevel, "log-level", "DEBUG", "")
	flags.StringVar(&nodeIDs, "node-id", "", "")
	flags.StringVar(&serverIDs, "server-id", "", "")
	flags.StringVar(&output, "output", "", "")
	flags.StringVar(&c.consulToken, "consul-token", "", "")
	flags.StringVar(&c.vaultToken, "vault-token", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Parse the time durations
	d, err := time.ParseDuration(duration)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing duration: %s: %s", duration, err.Error()))
		return 1
	}
	c.duration = d

	i, err := time.ParseDuration(interval)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing interval: %s: %s", interval, err.Error()))
		return 1
	}
	c.interval = i

	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err.Error()))
		return 1
	}

	// Resolve node prefixes
	for _, id := range argNodes(nodeIDs) {
		id = sanitizeUUIDPrefix(id)
		nodes, _, err := client.Nodes().PrefixList(id)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
			return 1
		}
		// Return error if no nodes are found
		if len(nodes) == 0 {
			c.Ui.Error(fmt.Sprintf("No node(s) with prefix %q found", id))
			return 1
		}

		for _, n := range nodes {
			c.nodeIDs = append(c.nodeIDs, n.ID)
		}
	}

	// Resolve server prefixes
	for _, id := range argNodes(serverIDs) {
		c.serverIDs = append(c.serverIDs, id)
	}

	c.manifest = make([]string, 0)
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.cancel = cancel

	// Setup the output path
	format := "2006-01-02-150405Z"
	c.timestamp = time.Now().UTC().Format(format)
	stamped := "nomad-debug-" + c.timestamp

	var tmp string
	if output != "" {
		tmp = filepath.Join(output, stamped)
		_, err := os.Stat(tmp)
		if !os.IsNotExist(err) {
			c.Ui.Error("Output directory already exists")
			return 2
		}
	} else {
		tmp, err = ioutil.TempDir(os.TempDir(), stamped)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating tmp directory: %s", err.Error()))
			return 2
		}
		defer os.RemoveAll(tmp)
	}

	c.collectDir = tmp

	// Capture signals so we can shutdown the monitor API calls on Int
	c.trap()

	err = c.collect(client)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error collecting data: %s", err.Error()))
		return 2
	}

	c.writeManifest()

	if output != "" {
		c.Ui.Output(fmt.Sprintf("Created debug directory: %s", c.collectDir))
		return 0
	}

	archiveFile := stamped + ".tar.gz"
	err = TarCZF(archiveFile, tmp, stamped)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating archive: %s", err.Error()))
		return 2
	}

	c.Ui.Output(fmt.Sprintf("Created debug archive: %s", archiveFile))
	return 0
}

// collect collects data from our endpoints and writes the archive bundle
func (c *DebugCommand) collect(client *api.Client) error {
	// Version contains cluster meta information
	dir := "version"
	err := c.mkdir(dir)
	if err != nil {
		return err
	}

	self, err := client.Agent().Self()
	if err != nil {
		return fmt.Errorf("agent self: %s", err.Error())
	}
	c.writeJSON(dir, "agent-self.json", self)

	// Fetch data directly from consul and vault. Ignore errors
	var consul, vault string

	m, ok := self.Config["Consul"].(map[string]interface{})
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

	m, ok = self.Config["Vault"].(map[string]interface{})
	if ok {
		raw := m["Addr"]
		vault, _ = raw.(string)
	}

	c.collectConsul(dir, consul)
	c.collectVault(dir, vault)

	c.startMonitors(client)
	c.collectPeriodic(client)
	c.collectPprofs(client)
	c.collectAgentHosts(client)

	return nil
}

// path returns platform specific paths in the tmp root directory
func (c *DebugCommand) path(paths ...string) string {
	ps := []string{c.collectDir}
	ps = append(ps, paths...)
	return filepath.Join(ps...)
}

// mkdir creates directories in the tmp root directory
func (c *DebugCommand) mkdir(paths ...string) error {
	return os.MkdirAll(c.path(paths...), 0755)
}

// startMonitors starts go routines for each node and client
func (c *DebugCommand) startMonitors(client *api.Client) {
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
func (c *DebugCommand) startMonitor(path, idKey, nodeID string, client *api.Client) {
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
func (c *DebugCommand) collectAgentHosts(client *api.Client) {
	for _, n := range c.nodeIDs {
		c.collectAgentHost("client", n, client)
	}

	for _, n := range c.serverIDs {
		c.collectAgentHost("server", n, client)
	}

}

// collectAgentHost gets the agent host data
func (c *DebugCommand) collectAgentHost(path, id string, client *api.Client) {
	var host *api.HostDataResponse
	var err error
	if path == "server" {
		host, err = client.Agent().Host(id, "", nil)
	} else {
		host, err = client.Agent().Host("", id, nil)
	}

	path = filepath.Join(path, id)

	if err != nil {
		c.writeBytes(path, "agent-host.log", []byte(err.Error()))
		return
	}

	c.writeJSON(path, "agent-host.json", host)
}

// collectPprofs captures the /agent/pprof for each listed node
func (c *DebugCommand) collectPprofs(client *api.Client) {
	for _, n := range c.nodeIDs {
		c.collectPprof("client", n, client)
	}

	for _, n := range c.serverIDs {
		c.collectPprof("server", n, client)
	}

}

// collectPprof captures pprof data for the node
func (c *DebugCommand) collectPprof(path, id string, client *api.Client) {
	opts := api.PprofOptions{Seconds: 1}
	if path == "server" {
		opts.ServerID = id
	} else {
		opts.NodeID = id
	}

	path = filepath.Join(path, id)

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
}

// collectPeriodic runs for duration, capturing the cluster state every interval. It flushes and stops
// the monitor requests
func (c *DebugCommand) collectPeriodic(client *api.Client) {
	// Not monitoring any logs, just capture the nomad context before exit
	if len(c.nodeIDs) == 0 && len(c.serverIDs) == 0 {
		dir := filepath.Join("nomad", "00")
		c.collectNomad(dir, client)
		return
	}

	duration := time.After(c.duration)
	// Set interval to 0 so that we immediately execute, wait the interval next time
	interval := time.After(0 * time.Second)
	var intervalCount int
	var dir string

	for {
		select {
		case <-duration:
			c.cancel()
			return

		case <-interval:
			dir = filepath.Join("nomad", fmt.Sprintf("%02d", intervalCount))
			c.collectNomad(dir, client)
			interval = time.After(c.interval)
			intervalCount += 1

		case <-c.ctx.Done():
			return
		}
	}
}

// collectNomad captures the nomad cluster state
func (c *DebugCommand) collectNomad(dir string, client *api.Client) error {
	err := c.mkdir(dir)
	if err != nil {
		return err
	}

	var qo *api.QueryOptions

	js, _, err := client.Jobs().List(qo)
	if err != nil {
		return fmt.Errorf("listing jobs: %s", err.Error())
	}
	c.writeJSON(dir, "jobs.json", js)

	ds, _, err := client.Deployments().List(qo)
	if err != nil {
		return fmt.Errorf("listing deployments: %s", err.Error())
	}
	c.writeJSON(dir, "deployments.json", ds)

	es, _, err := client.Evaluations().List(qo)
	if err != nil {
		return fmt.Errorf("listing evaluations: %s", err.Error())
	}
	c.writeJSON(dir, "evaluations.json", es)

	as, _, err := client.Allocations().List(qo)
	if err != nil {
		return fmt.Errorf("listing allocations: %s", err.Error())
	}
	c.writeJSON(dir, "allocations.json", as)

	ns, _, err := client.Nodes().List(qo)
	if err != nil {
		return fmt.Errorf("listing nodes: %s", err.Error())
	}
	c.writeJSON(dir, "nodes.json", ns)

	ps, _, err := client.CSIPlugins().List(qo)
	if err != nil {
		return fmt.Errorf("listing plugins: %s", err.Error())
	}
	c.writeJSON(dir, "plugins.json", ps)

	vs, _, err := client.CSIVolumes().List(qo)
	if err != nil {
		return fmt.Errorf("listing volumes: %s", err.Error())
	}
	c.writeJSON(dir, "volumes.json", vs)

	return nil
}

// collectConsul calls the Consul API directly to collect data
func (c *DebugCommand) collectConsul(dir, consul string) error {
	if consul == "" {
		return nil
	}

	token := c.consulToken
	if token == "" {
		token = os.Getenv("CONSUL_HTTP_TOKEN")
	}
	if token == "" {
		file := os.Getenv("CONSUL_HTTP_TOKEN_FILE")
		if file != "" {
			fh, err := os.Open(file)
			if err == nil {
				bs, err := ioutil.ReadAll(fh)
				if err == nil {
					token = strings.TrimSpace(string(bs))
				}
			}
		}
	}

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequest("GET", consul+"/v1/agent/self", nil)
	req.Header.Add("X-Consul-Token", token)
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	c.writeBody(dir, "consul-agent-self.json", resp, err)

	req, _ = http.NewRequest("GET", consul+"/v1/agent/members", nil)
	req.Header.Add("X-Consul-Token", token)
	req.Header.Add("User-Agent", userAgent)
	resp, err = client.Do(req)
	c.writeBody(dir, "consul-agent-members.json", resp, err)

	return nil
}

// collectVault calls the Vault API directly to collect data
func (c *DebugCommand) collectVault(dir, vault string) error {
	if vault == "" {
		return nil
	}

	token := c.vaultToken
	if token == "" {
		os.Getenv("VAULT_TOKEN")
	}

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequest("GET", vault+"/sys/health", nil)
	req.Header.Add("X-Vault-Token", token)
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	c.writeBody(dir, "vault-sys-health.json", resp, err)

	return nil
}

// writeBytes writes a file to the archive, recording it in the manifest
func (c *DebugCommand) writeBytes(dir, file string, data []byte) error {
	path := filepath.Join(dir, file)
	c.manifest = append(c.manifest, path)
	path = filepath.Join(c.collectDir, path)

	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = fh.Write(data)
	return err
}

// writeJSON writes JSON responses from the Nomad API calls to the archive
func (c *DebugCommand) writeJSON(dir, file string, data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.writeBytes(dir, file, bytes)
}

// writeBody is a helper that writes the body of an http.Response to the archive
func (c *DebugCommand) writeBody(dir, file string, resp *http.Response, err error) {
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.ContentLength == 0 {
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	c.writeBytes(dir, file, body)
}

// writeManifest creates the index files
func (c *DebugCommand) writeManifest() error {
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
func (c *DebugCommand) trap() {
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
