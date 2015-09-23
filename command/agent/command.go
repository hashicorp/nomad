package agent

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/nomad/helper/flag-slice"
	"github.com/hashicorp/nomad/helper/gated-writer"
	scada "github.com/hashicorp/scada-client"
	"github.com/mitchellh/cli"
)

// gracefulTimeout controls how long we wait before forcefully terminating
const gracefulTimeout = 5 * time.Second

// Command is a Command implementation that runs a Nomad agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Revision          string
	Version           string
	VersionPrerelease string
	Ui                cli.Ui
	ShutdownCh        <-chan struct{}

	args       []string
	agent      *Agent
	httpServer *HTTPServer
	logFilter  *logutils.LevelFilter
	logOutput  io.Writer

	scadaProvider *scada.Provider
	scadaHttp     *HTTPServer
}

func (c *Command) readConfig() *Config {
	var dev bool
	var configPath []string

	// Make a new, empty config.
	cmdConfig := &Config{
		Atlas:  &AtlasConfig{},
		Client: &ClientConfig{},
		Ports:  &Ports{},
		Server: &ServerConfig{},
	}

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Error(c.Help()) }

	// Role options
	flags.BoolVar(&dev, "dev", false, "")
	flags.BoolVar(&cmdConfig.Server.Enabled, "server", false, "")
	flags.BoolVar(&cmdConfig.Client.Enabled, "client", false, "")

	// Server-only options
	flags.IntVar(&cmdConfig.Server.BootstrapExpect, "bootstrap-expect", 0, "")

	// General options
	flags.Var((*sliceflag.StringFlag)(&configPath), "config", "config")
	flags.StringVar(&cmdConfig.BindAddr, "bind", "", "")
	flags.StringVar(&cmdConfig.Region, "region", "", "")
	flags.StringVar(&cmdConfig.DataDir, "data-dir", "", "")
	flags.StringVar(&cmdConfig.Datacenter, "dc", "", "")
	flags.StringVar(&cmdConfig.LogLevel, "log-level", "info", "")
	flags.StringVar(&cmdConfig.NodeName, "node", "", "")

	// Atlas options
	flags.StringVar(&cmdConfig.Atlas.Infrastructure, "atlas", "", "")
	flags.BoolVar(&cmdConfig.Atlas.Join, "atlas-join", false, "")
	flags.StringVar(&cmdConfig.Atlas.Token, "atlas-token", "", "")

	if err := flags.Parse(c.args); err != nil {
		return nil
	}

	// Load the configuration
	var config *Config
	if dev {
		config = DevConfig()
	} else {
		config = DefaultConfig()
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

	// Ensure the sub-structs at least exist
	if config.Atlas == nil {
		config.Atlas = &AtlasConfig{}
	}
	if config.Client == nil {
		config.Client = &ClientConfig{}
	}
	if config.Server == nil {
		config.Server = &ServerConfig{}
	}

	// Merge any CLI options over config file options
	config = config.Merge(cmdConfig)

	// Set the version info
	config.Revision = c.Revision
	config.Version = c.Version
	config.VersionPrerelease = c.VersionPrerelease

	if dev {
		// Skip validation for dev mode
		return config
	}

	// Check that we have a data-dir
	if config.DataDir == "" {
		c.Ui.Error("Must specify data directory")
		return nil
	}

	// Check the bootstrap flags
	if config.Server.BootstrapExpect > 0 && !config.Server.Enabled {
		c.Ui.Error("Bootstrap requires server mode to be enabled")
		return nil
	}
	if config.Server.BootstrapExpect == 1 {
		c.Ui.Error("WARNING: Bootstrap mode enabled! Potentially unsafe operation.")
	}

	return config
}

// setupLoggers is used to setup the logGate, logWriter, and our logOutput
func (c *Command) setupLoggers(config *Config) (*gatedwriter.Writer, *logWriter, io.Writer) {
	// Setup logging. First create the gated log writer, which will
	// store logs until we're ready to show them. Then create the level
	// filter, filtering logs of the specified level.
	logGate := &gatedwriter.Writer{
		Writer: &cli.UiWriter{Ui: c.Ui},
	}

	c.logFilter = LevelFilter()
	c.logFilter.MinLevel = logutils.LogLevel(strings.ToUpper(config.LogLevel))
	c.logFilter.Writer = logGate
	if !ValidateLevelFilter(c.logFilter.MinLevel, c.logFilter) {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			c.logFilter.MinLevel, c.logFilter.Levels))
		return nil, nil, nil
	}

	// Check if syslog is enabled
	var syslog io.Writer
	if config.EnableSyslog {
		l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, config.SyslogFacility, "nomad")
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Syslog setup failed: %v", err))
			return nil, nil, nil
		}
		syslog = &SyslogWrapper{l, c.logFilter}
	}

	// Create a log writer, and wrap a logOutput around it
	logWriter := NewLogWriter(512)
	var logOutput io.Writer
	if syslog != nil {
		logOutput = io.MultiWriter(c.logFilter, logWriter, syslog)
	} else {
		logOutput = io.MultiWriter(c.logFilter, logWriter)
	}
	c.logOutput = logOutput
	return logGate, logWriter, logOutput
}

// setupAgent is used to start the agent and various interfaces
func (c *Command) setupAgent(config *Config, logOutput io.Writer) error {
	c.Ui.Output("Starting Nomad agent...")
	agent, err := NewAgent(config, logOutput)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting agent: %s", err))
		return err
	}
	c.agent = agent

	// Enable the SCADA integration
	if err := c.setupSCADA(config); err != nil {
		agent.Shutdown()
		c.Ui.Error(fmt.Sprintf("Error starting SCADA: %s", err))
		return err
	}

	// Setup the HTTP server
	http, err := NewHTTPServer(agent, config, logOutput)
	if err != nil {
		agent.Shutdown()
		c.Ui.Error(fmt.Sprintf("Error starting http server: %s", err))
		return err
	}
	c.httpServer = http

	// Setup update checking
	if !config.DisableUpdateCheck {
		version := config.Version
		if config.VersionPrerelease != "" {
			version += fmt.Sprintf("-%s", config.VersionPrerelease)
		}
		updateParams := &checkpoint.CheckParams{
			Product: "nomad",
			Version: version,
		}
		if !config.DisableAnonymousSignature {
			updateParams.SignatureFile = filepath.Join(config.DataDir, "checkpoint-signature")
		}

		// Schedule a periodic check with expected interval of 24 hours
		checkpoint.CheckInterval(updateParams, 24*time.Hour, c.checkpointResults)

		// Do an immediate check within the next 30 seconds
		go func() {
			time.Sleep(randomStagger(30 * time.Second))
			c.checkpointResults(checkpoint.Check(updateParams))
		}()
	}
	return nil
}

// checkpointResults is used to handler periodic results from our update checker
func (c *Command) checkpointResults(results *checkpoint.CheckResponse, err error) {
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}
	if results.Outdated {
		c.Ui.Error(fmt.Sprintf("Newer Nomad version available: %s", results.CurrentVersion))
	}
	for _, alert := range results.Alerts {
		switch alert.Level {
		case "info":
			c.Ui.Info(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
		default:
			c.Ui.Error(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
		}
	}
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

	// Setup the log outputs
	logGate, _, logOutput := c.setupLoggers(config)
	if logGate == nil {
		return 1
	}

	// Initialize the telemetry
	if err := c.setupTelementry(config); err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing telemetry: %s", err))
		return 1
	}

	// Create the agent
	if err := c.setupAgent(config, logOutput); err != nil {
		return 1
	}
	defer c.agent.Shutdown()

	// Check and shut down the SCADA listeners at the end
	defer func() {
		if c.httpServer != nil {
			c.httpServer.Shutdown()
		}
		if c.scadaHttp != nil {
			c.scadaHttp.Shutdown()
		}
		if c.scadaProvider != nil {
			c.scadaProvider.Shutdown()
		}
	}()

	// Compile agent information for output later
	info := make(map[string]string)
	info["client"] = strconv.FormatBool(config.Client.Enabled)
	info["log level"] = config.LogLevel
	info["server"] = strconv.FormatBool(config.Server.Enabled)
	info["region"] = fmt.Sprintf("%s (DC: %s)", config.Region, config.Datacenter)
	if config.Atlas != nil && config.Atlas.Infrastructure != "" {
		info["atlas"] = fmt.Sprintf("(Infrastructure: '%s' Join: %v)",
			config.Atlas.Infrastructure, config.Atlas.Join)
	} else {
		info["atlas"] = "<disabled>"
	}

	// Sort the keys for output
	infoKeys := make([]string, 0, len(info))
	for key := range info {
		infoKeys = append(infoKeys, key)
	}
	sort.Strings(infoKeys)

	// Agent configuration output
	padding := 18
	c.Ui.Output("Nomad agent configuration:\n")
	for _, k := range infoKeys {
		c.Ui.Info(fmt.Sprintf(
			"%s%s: %s",
			strings.Repeat(" ", padding-len(k)),
			strings.Title(k),
			info[k]))
	}
	c.Ui.Output("")

	// Output the header that the server has started
	c.Ui.Output("Nomad agent started! Log data will stream in below:\n")

	// Enable log streaming
	logGate.Flush()

	// Wait for exit
	return c.handleSignals(config)
}

// handleSignals blocks until we get an exit-causing signal
func (c *Command) handleSignals(config *Config) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for a signal
WAIT:
	var sig os.Signal
	select {
	case s := <-signalCh:
		sig = s
	case <-c.ShutdownCh:
		sig = os.Interrupt
	}
	c.Ui.Output(fmt.Sprintf("Caught signal: %v", sig))

	// Check if this is a SIGHUP
	if sig == syscall.SIGHUP {
		if conf := c.handleReload(config); conf != nil {
			config = conf
		}
		goto WAIT
	}

	// Check if we should do a graceful leave
	graceful := false
	if sig == os.Interrupt && config.LeaveOnInt {
		graceful = true
	} else if sig == syscall.SIGTERM && config.LeaveOnTerm {
		graceful = true
	}

	// Bail fast if not doing a graceful leave
	if !graceful {
		return 1
	}

	// Attempt a graceful leave
	gracefulCh := make(chan struct{})
	c.Ui.Output("Gracefully shutting down agent...")
	go func() {
		if err := c.agent.Leave(); err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return
		}
		close(gracefulCh)
	}()

	// Wait for leave or another signal
	select {
	case <-signalCh:
		return 1
	case <-time.After(gracefulTimeout):
		return 1
	case <-gracefulCh:
		return 0
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (c *Command) handleReload(config *Config) *Config {
	c.Ui.Output("Reloading configuration...")
	newConf := c.readConfig()
	if newConf == nil {
		c.Ui.Error(fmt.Sprintf("Failed to reload configs"))
		return config
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newConf.LogLevel))
	if ValidateLevelFilter(minLevel, c.logFilter) {
		c.logFilter.SetMinLevel(minLevel)
	} else {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, c.logFilter.Levels))

		// Keep the current log level
		newConf.LogLevel = config.LogLevel
	}
	return newConf
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

	metricsConf := metrics.DefaultConfig("nomad")
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

// setupSCADA is used to start a new SCADA provider and listener,
// replacing any existing listeners.
func (c *Command) setupSCADA(config *Config) error {
	// Shut down existing SCADA listeners
	if c.scadaProvider != nil {
		c.scadaProvider.Shutdown()
	}
	if c.scadaHttp != nil {
		c.scadaHttp.Shutdown()
	}

	// No-op if we don't have an infrastructure
	if config.Atlas == nil || config.Atlas.Infrastructure == "" {
		return nil
	}

	// Create the new provider and listener
	c.Ui.Output("Connecting to Atlas: " + config.Atlas.Infrastructure)
	provider, list, err := NewProvider(config, c.logOutput)
	if err != nil {
		return err
	}
	c.scadaProvider = provider
	c.scadaHttp = newScadaHttp(c.agent, list)
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

  The Nomad agent's configuration primarily comes from the config
  files used, but a subset of the options may also be passed directly
  as CLI arguments, listed below.

General Options (clients and servers):

  -bind=<addr>
    The address the agent will bind to for all of its various network
    services. The individual services that run bind to individual
    ports on this address. Defaults to the loopback 127.0.0.1.

  -config=<path>
    The path to either a single config file or a directory of config
    files to use for configuring the Nomad agent. This option may be
    specified multiple times. If multiple config files are used, the
    values from each will be merged together. During merging, values
    from files found later in the list are merged over values from
    previously parsed files.

  -data-dir=<path>
    The data directory used to store state and other persistent data.
    On client machines this is used to house allocation data such as
    downloaded artifacts used by drivers. On server nodes, the data
    dir is also used to store the replicated log.

  -dc=<datacenter>
    The name of the datacenter this Nomad agent is a member of. By
    default this is set to "dc1".

  -log-level=<level>
    Specify the verbosity level of Nomad's logs. Valid values include
    DEBUG, INFO, and WARN, in decreasing order of verbosity. The
    default is INFO.

  -node=<name>
    The name of the local agent. This name is used to identify the node
    in the cluster. The name must be unique per region. The default is
    the current hostname of the machine.

  -region=<region>
    Name of the region the Nomad agent will be a member of. By default
    this value is set to "global".

  -dev
    Start the agent in development mode. This enables a pre-configured
    dual-role agent (client + server) which is useful for developing
    or testing Nomad. No other configuration is required to start the
    agent in this mode.

Server Options:

  -server
    Enable server mode for the agent. Agents in server mode are
    clustered together and handle the additional responsibility of
    leader election, data replication, and scheduling work onto
    eligible client nodes.

  -bootstrap-expect=<num>
    Configures the expected number of servers nodes to wait for before
    bootstrapping the cluster. Once <num> servers have joined eachother,
    Nomad initiates the bootstrap process.

Client Options:

  -client
    Enable client mode for the agent. Client mode enables a given node
    to be evaluated for allocations. If client mode is not enabled,
    no work will be scheduled to the agent.

Atlas Options:

  -atlas=<infrastructure>
    The Atlas infrastructure name to configure. This enables the SCADA
    client and attempts to connect Nomad to the HashiCorp Atlas service
    using the provided infrastructure name and token.

  -atlas-token=<token>
    The Atlas token to use when connecting to the HashiCorp Atlas
    service. This must be provided to successfully connect your Nomad
    agent to Atlas.

  -atlas-join
    Enable the Atlas join feature. This mode allows agents to discover
    eachother automatically using the SCADA integration features.
 `
	return strings.TrimSpace(helpText)
}
