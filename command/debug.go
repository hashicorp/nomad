package command

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type DebugCommand struct {
	Meta
}

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

  -job <job> [job]
    Select the context for the job, and any client nodes running the job.

  -node <node> [node]
    Select the context for the node and all jobs running on the node.

  -plugin <plugin> [plugin]
    Select the context for the plugin and all jobs providing it.

  -volume <volume> [volume]
    Select the context for the volume and all jobs providing its plugins or
    using the volume, and nodes where the volume is mounted.
`
	return strings.TrimSpace(helpText)
}

func (c *DebugCommand) Synopsis() string {
	return "Build a debug archive"
}

func (c *DebugCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-job":    complete.PredictAnything,
			"-node":   complete.PredictAnything,
			"-plugin": complete.PredictAnything,
			"-volume": complete.PredictAnything,
		})
}

func (c *DebugCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *DebugCommand) Name() string { return "debug" }

func (c *DebugCommand) Run(args []string) int {
	tmp, err := ioutil.TempDir(os.TempDir(), "nomad-debug-")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error creating tmp directory: %s", err.Error()))
		return 2
	}
	defer os.RemoveAll(tmp)

	err = c.collect(tmp)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error collecting data: %s", err.Error()))
		return 2
	}

	format := "2006-01-02-150405Z"
	archive := "nomad-debug-" + time.Now().UTC().Format(format) + ".tar.gz"
	err = TarCZF(archive, tmp)
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

	// consul := self.Config["consul"]
	// vaultMap := self.Config["vault"].(map[string]interface{})
	// vault := vaultMap["addr"]

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

func writeBytes(path string, data []byte) error {
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
	file = filepath.Join(dir, file)
	return writeBytes(file, bytes)
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
