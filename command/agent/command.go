package agent

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/vault/helper/flag-slice"
	"github.com/mitchellh/cli"
)

// Command is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Revision          string
	Version           string
	VersionPrerelease string
	Ui                cli.Ui
	ShutdownCh        <-chan struct{}

	args []string
}

func (c *Command) readConfig() *Config {
	var dev bool
	var configPath []string
	var logLevel string
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.BoolVar(&dev, "dev", false, "")
	flags.StringVar(&logLevel, "log-level", "info", "")
	flags.Usage = func() { c.Ui.Error(c.Help()) }
	flags.Var((*sliceflag.StringFlag)(&configPath), "config", "config")
	if err := flags.Parse(c.args); err != nil {
		return nil
	}

	// Load the configuration
	var config *Config
	if dev {
		config = DevConfig()
	}
	for _, path := range configPath {
		current, err := LoadConfig(path)
		if err != nil {
			c.Ui.Error(fmt.Sprintf(
				"Error loading configuration from %s: %s", path, err))
			return nil
		}

		if config == nil {
			config = current
		} else {
			config = config.Merge(current)
		}
	}

	return config
}

func (c *Command) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	// Parse our configs
	c.args = args
	config := c.readConfig()
	if config == nil {
		return 1
	}

	// Initialize the telemetry
	if err := c.setupTelementry(config); err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing telemetry: %s", err))
		return 1
	}

	return 0
}

// setupTelementry is used ot setup the telemetry sub-systems
func (c *Command) setupTelementry(config *Config) error {
	/* Setup telemetry
	Aggregate on 10 second intervals for 1 minute. Expose the
	metrics over stderr when there is a SIGUSR1 received.
	*/
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)

	var telConfig *Telemetry
	if config.Telemetry == nil {
		telConfig = &Telemetry{}
	} else {
		telConfig = config.Telemetry
	}

	metricsConf := metrics.DefaultConfig("vault")
	metricsConf.EnableHostname = !telConfig.DisableHostname

	// Configure the statsite sink
	var fanout metrics.FanoutSink
	if telConfig.StatsiteAddr != "" {
		sink, err := metrics.NewStatsiteSink(telConfig.StatsiteAddr)
		if err != nil {
			return err
		}
		fanout = append(fanout, sink)
	}

	// Configure the statsd sink
	if telConfig.StatsdAddr != "" {
		sink, err := metrics.NewStatsdSink(telConfig.StatsdAddr)
		if err != nil {
			return err
		}
		fanout = append(fanout, sink)
	}

	// Initialize the global sink
	if len(fanout) > 0 {
		fanout = append(fanout, inm)
		metrics.NewGlobal(metricsConf, fanout)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, inm)
	}
	return nil
}

func (c *Command) Synopsis() string {
	return "Runs a Nomad agent"
}

func (c *Command) Help() string {
	helpText := `
Usage: nomad agent [options]

  Starts the Nomad agent and runs until an interrupt is received.
  The agent may be a client and/or server.

Options:

  -advertise=addr          Sets the advertise address to use
  -atlas=org/name          Sets the Atlas infrastructure name, enables SCADA.
  -atlas-join              Enables auto-joining the Atlas cluster
  -atlas-token=token       Provides the Atlas API token
  -bootstrap               Sets server to bootstrap mode
  -bind=0.0.0.0            Sets the bind address for cluster communication
  -bootstrap-expect=0      Sets server to expect bootstrap mode.
  -client=127.0.0.1        Sets the address to bind for client access.
                           This includes RPC, DNS, HTTP and HTTPS (if configured)
  -config-file=foo         Path to a JSON file to read configuration from.
                           This can be specified multiple times.
  -config-dir=foo          Path to a directory to read configuration files
                           from. This will read every file ending in ".json"
                           as configuration in this directory in alphabetical
                           order. This can be specified multiple times.
  -data-dir=path           Path to a data directory to store agent state
  -recursor=1.2.3.4        Address of an upstream DNS server.
                           Can be specified multiple times.
  -dc=east-aws             Datacenter of the agent
  -encrypt=key             Provides the gossip encryption key
  -join=1.2.3.4            Address of an agent to join at start time.
                           Can be specified multiple times.
  -join-wan=1.2.3.4        Address of an agent to join -wan at start time.
                           Can be specified multiple times.
  -retry-join=1.2.3.4      Address of an agent to join at start time with
                           retries enabled. Can be specified multiple times.
  -retry-interval=30s      Time to wait between join attempts.
  -retry-max=0             Maximum number of join attempts. Defaults to 0, which
                           will retry indefinitely.
  -retry-join-wan=1.2.3.4  Address of an agent to join -wan at start time with
                           retries enabled. Can be specified multiple times.
  -retry-interval-wan=30s  Time to wait between join -wan attempts.
  -retry-max-wan=0         Maximum number of join -wan attempts. Defaults to 0, which
                           will retry indefinitely.
  -log-level=info          Log level of the agent.
  -node=hostname           Name of this node. Must be unique in the cluster
  -protocol=N              Sets the protocol version. Defaults to latest.
  -rejoin                  Ignores a previous leave and attempts to rejoin the cluster.
  -server                  Switches agent to server mode.
  -syslog                  Enables logging to syslog
  -ui-dir=path             Path to directory containing the Web UI resources
  -pid-file=path           Path to file to store agent PID
 `
	return strings.TrimSpace(helpText)
}
