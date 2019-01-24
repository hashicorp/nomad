//+build linux

package nomad09upgrade

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	getter "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "nomad09upgrade",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			&UpgradePathTC{},
		},
	})
}

var (
	nomadVersions = []string{
		"0.8.7",
		"0.8.6",
		"0.8.5",
		"0.8.4",
		"0.8.3",
		"0.8.2",
		"0.8.1",
		"0.8.0",
	}

	agentTemplate = `
ports {
  http = {{.HTTP}}
  rpc  = {{.RPC}}
  serf = {{.Serf}}
}

server {
	enabled = true
	bootstrap_expect = 1
}

client {
	enabled = true
	options = {
      "driver.raw_exec.enable" = "1"
    }
}
`
)

type templateVars struct {
	HTTP int
	RPC  int
	Serf int
}

func writeConfig(path string, vars templateVars) error {
	t := template.Must(template.New("config").Parse(agentTemplate))
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, vars)
}

type UpgradePathTC struct {
	framework.TC

	binDir string
	bin    string
}

type nomadAgent struct {
	bin     string
	dataDir string
	cmd     *exec.Cmd
	vars    templateVars

	targetCmd *exec.Cmd
}

func (tc *UpgradePathTC) newNomadServer(t *testing.T, ver string) (*nomadAgent, error) {
	dir, err := ioutil.TempDir("/opt", "nomade2e")
	if err != nil {
		return nil, err
	}

	return tc.newNomadServerWithDataDir(t, ver, dir)
}

func (tc *UpgradePathTC) newNomadServerWithDataDir(t *testing.T, ver string, dataDir string) (*nomadAgent, error) {
	srv := &nomadAgent{
		bin:     filepath.Join(tc.binDir, ver, "nomad"),
		dataDir: dataDir,
	}
	conf := filepath.Join(dataDir, "config.hcl")
	httpPort, err := getFreePort()
	if err != nil {
		return nil, err
	}
	rpcPort, err := getFreePort()
	if err != nil {
		return nil, err
	}
	serfPort, err := getFreePort()
	if err != nil {
		return nil, err
	}

	srv.vars = templateVars{
		HTTP: httpPort,
		RPC:  rpcPort,
		Serf: serfPort,
	}
	if err := writeConfig(conf, srv.vars); err != nil {
		return nil, err
	}

	w := testlog.NewWriter(t)

	srv.cmd = exec.Command(srv.bin, "agent", "-config", conf, "-log-level", "DEBUG", "-data-dir", srv.dataDir)
	srv.cmd.Stdout = w
	srv.cmd.Stderr = w

	srv.targetCmd = exec.Command(tc.bin, "agent", "-config", conf, "-log-level", "TRACE", "-data-dir", srv.dataDir)
	srv.targetCmd.Stdout = w
	srv.targetCmd.Stderr = w
	return srv, nil
}

func (n *nomadAgent) StartAgent() error {
	return n.cmd.Start()
}

func (n *nomadAgent) StopAgent() error {
	if err := n.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	n.cmd.Wait()
	return nil
}

func (n *nomadAgent) StartTargetAgent() error {
	return n.targetCmd.Start()
}

func (n *nomadAgent) StopTargetAgent() error {
	if err := n.targetCmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	n.targetCmd.Wait()
	return nil
}

func (n *nomadAgent) Destroy() {
	os.RemoveAll(n.dataDir)
}

func (n *nomadAgent) Nomad() (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = fmt.Sprintf("http://127.0.0.1:%d", n.vars.HTTP)
	return api.NewClient(cfg)
}

// BeforeAll downloads all of the desired nomad versions to test against
func (tc *UpgradePathTC) BeforeAll(f *framework.F) {
	bin, err := discover.NomadExecutable()
	f.NoError(err)
	tc.bin = bin
	dir, err := ioutil.TempDir("", "")
	f.NoError(err)

	tc.binDir = dir
	for _, ver := range nomadVersions {
		verBin := filepath.Join(tc.binDir, ver)
		f.NoError(os.Mkdir(verBin, 0755))
		f.NoError(
			getter.Get(verBin, fmt.Sprintf(
				"https://releases.hashicorp.com/nomad/%s/nomad_%s_linux_amd64.zip",
				ver, ver,
			)))
		f.T().Logf("downloaded nomad version %s to %s", ver, verBin)
	}
}

// AfterAll cleans up the downloaded nomad binaries
func (tc *UpgradePathTC) AfterAll(f *framework.F) {
	os.RemoveAll(tc.binDir)
}

func (tc *UpgradePathTC) TestRawExecTaskUpgrade(f *framework.F) {
	for _, ver := range nomadVersions {
		ver := ver
		f.T().Run(ver, func(t *testing.T) {
			t.Parallel()
			tc.testUpgradeForJob(t, ver, "nomad09upgrade/rawexec.nomad")
		})
	}
}

func (tc *UpgradePathTC) TestExecTaskUpgrade(f *framework.F) {
	for _, ver := range nomadVersions {
		ver := ver
		f.T().Run(ver, func(t *testing.T) {
			t.Parallel()
			tc.testUpgradeForJob(t, ver, "nomad09upgrade/exec.nomad")
		})
	}
}

func (tc *UpgradePathTC) TestDockerTaskUpgrade(f *framework.F) {
	for _, ver := range nomadVersions {
		ver := ver
		f.T().Run(ver, func(t *testing.T) {
			t.Parallel()
			tc.testUpgradeForJob(t, ver, "nomad09upgrade/docker.nomad")
		})
	}
}

func (tc *UpgradePathTC) testUpgradeForJob(t *testing.T, ver string, jobfile string) {
	require := require.New(t)
	// Start a nomad agent for the given version
	srv, err := tc.newNomadServer(t, ver)
	require.NoError(err)
	t.Logf("launching v%s nomad agent", ver)
	require.NoError(srv.StartAgent())

	// Wait for the agent to be ready
	client, err := srv.Nomad()
	require.NoError(err)
	e2eutil.WaitForNodesReady(t, client, 1)

	// Register a sleep job
	jobID := "sleep-" + uuid.Generate()[:8]
	t.Logf("registering exec job with id %s", jobID)
	e2eutil.RegisterAndWaitForAllocs(t, client, jobfile, jobID)
	allocs, _, err := client.Jobs().Allocations(jobID, false, nil)
	require.NoError(err)
	require.Len(allocs, 1)

	// Wait for sleep job to transition to running
	id := allocs[0].ID
	e2eutil.WaitForAllocRunning(t, client, id)

	// Stop the agent, leaving the sleep job running
	require.NoError(srv.StopAgent())

	// Start a nomad agent with the to be tested nomad binary
	t.Logf("launching test nomad agent")
	require.NoError(srv.StartTargetAgent())

	// Wait for the agent to be ready
	client, err = srv.Nomad()
	require.NoError(err)
	e2eutil.WaitForNodesReady(t, client, 1)

	// Make sure the same allocation still exists
	alloc, _, err := client.Allocations().Info(id, nil)
	require.NoError(err)
	// Pull stats from the allocation, testing that new code can interface with
	// the old stats driver apis
	testutil.WaitForResult(func() (bool, error) {
		stats, err := client.Allocations().Stats(alloc, nil)
		if err != nil {
			return false, err
		}

		return stats.ResourceUsage.MemoryStats.RSS > 0, fmt.Errorf("RSS for task should be greater than 0")
	}, func(err error) {
		require.NoError(err)
	})

	// Deregister the job. This tests that the new code can properly tear down
	// upgraded allocs
	_, _, err = client.Jobs().Deregister(alloc.JobID, true, nil)
	require.NoError(err)
	testutil.WaitForResult(func() (bool, error) {
		j, _, _ := client.Jobs().Info(jobID, nil)
		if j == nil {
			return true, nil
		}
		time.Sleep(time.Millisecond * 100)
		return false, fmt.Errorf("job with id %q should be purged", jobID)
	}, func(err error) {
		require.NoError(err)
	})

	// Check that the task dir mounts have been removed
	testutil.WaitForResult(func() (bool, error) {
		defer client.System().GarbageCollect()
		time.Sleep(time.Millisecond * 100)
		data, err := ioutil.ReadFile("/proc/mounts")
		if err != nil {
			return false, err
		}

		return !strings.Contains(string(data), id), fmt.Errorf("taskdir mounts should be cleaned up, but found mount for id %q:\n%s", id, string(data))
	}, func(err error) {
		require.NoError(err)
	})

	// Cleanup
	srv.StopTargetAgent()
	srv.Destroy()
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
