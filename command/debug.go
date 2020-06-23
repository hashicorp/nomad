package command

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type DebugCommand struct {
	Meta
	duration    time.Duration
	interval    time.Duration
	logLevel    string
	nodeIDs     []string
	consulToken string
	vaultToken  string
}

const (
	userAgent = "nomad debug"
)

func (c *DebugCommand) Help() string {
	helpText := `
Usage: nomad debug [options]

  Build an archive containing Nomad cluster configuration and state information, Nomad
  server and task logs, and logs from the host systems running Nomad servers and clients. If
  no selection option is provided, the debug archive contains logs from all nodes in the
  cluster. Multiple selector options may be provided.

General Options:

  ` + generalOptionsUsage() + `

Debug Options:

  -duration=2m
   The duration of the log capture command. Defaults to 2m.

  -interval=2m
   The interval between snapshots of the Nomad state. If unspecified, only one snapshot is
   captured.

  -log-level=DEBUG
   The log level of logs to capture. Defaults to DEBUG.

  -node-id=n1,n2
   Comma seperated list of Nomad client node ids, for which we capture logs and pprof data.
   Accepts id prefixes.

  -server-id=s1,s2
   Comma seperated list of Nomad server node ids, for which we capture logs and pprof data.
   Accepts id prefixes.

  -output=path
   Path to the parent directory of the output directory. Defaults to the current directory.

  -archive
   Build an archive and delete the output directory. Defaults to true.

  -consul-token
   Token use to query Consul. Defaults to CONSUL_TOKEN

  -vault-token
   Token use to query Vault. Defaults to VAULT_TOKEN
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
			"-archive":      complete.PredictAnything,
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
	var archive bool

	flags.StringVar(&duration, "duration", "2m", "")
	flags.StringVar(&interval, "interval", "2m", "")
	flags.StringVar(&c.logLevel, "log-level", "", "")
	flags.StringVar(&nodeIDs, "node-id", "", "")
	flags.StringVar(&serverIDs, "server-id", "", "")
	flags.StringVar(&output, "output", "", "")
	flags.BoolVar(&archive, "archive", true, "")
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

	// Setup the output path
	format := "2006-01-02-150405Z"
	stamped := "nomad-debug-" + time.Now().UTC().Format(format)

	var tmp string
	if output != "" {
		tmp = filepath.Join(output, stamped)
	} else {
		tmp, err = ioutil.TempDir(os.TempDir(), stamped)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating tmp directory: %s", err.Error()))
			return 2
		}
		defer os.RemoveAll(tmp)
	}

	err = c.collect(tmp)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error collecting data: %s", err.Error()))
		return 2
	}

	// FIXME does output imply no archive?
	if !archive || output != "" {
		return 0
	}

	err = TarCZF(stamped+".tar.gz", tmp)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error creating archive: %s", err.Error()))
		return 2
	}

	return 0
}

// collect collects data from our endpoints and writes the archive bundle
func (c *DebugCommand) collect(root string) error {
	client, err := c.Meta.Client()
	if err != nil {
		return fmt.Errorf("Error initializing client: %s", err.Error())
	}

	// Version contains cluster meta information
	dir := filepath.Join(root, "version")
	err = os.Mkdir(dir, 0755)
	if err != nil {
		return err
	}

	self, err := client.Agent().Self()
	if err != nil {
		return fmt.Errorf("error agent self: %s", err.Error())
	}
	writeJSON(dir, "agent-self.json", self)

	// Fetch data directly from consul and vault. Ignore errors
	var consul, vault string
	consulRaw := self.Config["consul"]
	consul, _ = consulRaw.(string)
	vaultMap, ok := self.Config["vault"].(map[string]interface{})
	if ok {
		vaultRaw := vaultMap["addr"]
		vault, _ = vaultRaw.(string)
	}
	c.collectConsul(dir, consul)
	c.collectVault(dir, vault)

	// For each server, collect the agent host state
	dir = filepath.Join(root, "server")
	err = os.Mkdir(dir, 0755)
	if err != nil {
		return err
	}

	var qo *api.QueryOptions

	hostdata, _, err := client.Operator().ServerHosts(qo)
	writeJSON(dir, "operator-server-hosts.json", hostdata)

	// Nomad contains nomad cluster state
	dir = filepath.Join(root, "nomad")
	err = os.Mkdir(dir, 0755)
	if err != nil {
		return err
	}

	js, _, err := client.Jobs().List(qo)
	if err != nil {
		return fmt.Errorf("error listing jobs: %s", err.Error())
	}
	writeJSON(dir, "jobs.json", js)

	ds, _, err := client.Deployments().List(qo)
	if err != nil {
		return fmt.Errorf("error listing deployments: %s", err.Error())
	}
	writeJSON(dir, "deployments.json", ds)

	es, _, err := client.Evaluations().List(qo)
	if err != nil {
		return fmt.Errorf("error listing evaluations: %s", err.Error())
	}
	writeJSON(dir, "evaluations.json", es)

	as, _, err := client.Allocations().List(qo)
	if err != nil {
		return fmt.Errorf("error listing allocations: %s", err.Error())
	}
	writeJSON(dir, "allocations.json", as)

	ns, _, err := client.Nodes().List(qo)
	if err != nil {
		return fmt.Errorf("error listing nodes: %s", err.Error())
	}
	writeJSON(dir, "nodes.json", ns)

	ps, _, err := client.CSIPlugins().List(qo)
	if err != nil {
		return fmt.Errorf("error listing plugins: %s", err.Error())
	}
	writeJSON(dir, "plugins.json", ps)

	vs, _, err := client.CSIVolumes().List(qo)
	if err != nil {
		return fmt.Errorf("error listing volumes: %s", err.Error())
	}
	writeJSON(dir, "volumes.json", vs)

	return nil
}

func (c *DebugCommand) collectLogs(root string) error {
	return nil
}

// collectConsul calls the consul api directly to collect data
func (c *DebugCommand) collectConsul(root, consul string) error {
	if consul == "" {
		return nil
	}

	token := c.consulToken
	if token == "" {
		os.Getenv("CONSUL_TOKEN")
	}

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequest("GET", consul+"/agent/self", nil)
	req.Header.Add("X-Consul-Token", token)
	req.Header.Add("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err == nil {
		body := make([]byte, resp.ContentLength)
		resp.Body.Read(body)
		writeBytes(root, "consul-agent-self.json", body)
	}

	req, _ = http.NewRequest("GET", consul+"/agent/members", nil)
	req.Header.Add("X-Consul-Token", token)
	req.Header.Add("User-Agent", userAgent)
	resp, err = client.Do(req)
	if err == nil {
		body := make([]byte, resp.ContentLength)
		resp.Body.Read(body)
		writeBytes(root, "consul-agent-members.json", body)
	}

	return nil
}

func (c *DebugCommand) collectVault(root, vault string) error {
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
	resp, _ := client.Do(req)
	body := make([]byte, resp.ContentLength)
	resp.Body.Read(body)
	writeBytes(root, "consul-agent-self.json", body)

	return nil
}

func writeBytes(dir, file string, data []byte) error {
	path := filepath.Join(dir, file)
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = fh.Write(data)
	return err
}

func writeJSON(dir, file string, data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return writeBytes(dir, file, bytes)
}

// TarCZF, like the tar command, recursively builds a gzip compressed tar archive from a
// directory
func TarCZF(archive string, src string) error {
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
		header.Name = strings.TrimPrefix(strings.Replace(file, src, "", -1), string(filepath.Separator))

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
