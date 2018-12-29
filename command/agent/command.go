package agent

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/consul/lib"
	checkpoint "github.com/hashicorp/go-checkpoint"
	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/nomad/helper/flag-helpers"
	"github.com/hashicorp/nomad/helper/gated-writer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/version"
	"github.com/hashicorp/scada-client/scada"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// gracefulTimeout controls how long we wait before forcefully terminating
const gracefulTimeout = 5 * time.Second

// Command is a Command implementation that runs a Nomad agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Version    *version.VersionInfo
	Ui         cli.Ui
	ShutdownCh <-chan struct{}

	args           []string
	agent          *Agent
	httpServer     *HTTPServer
	logFilter      *logutils.LevelFilter
	logOutput      io.Writer
	retryJoinErrCh chan struct{}

	scadaProvider *scada.Provider
	scadaHttp     *HTTPServer
}

func (c *Command) readConfig() *Config {
	var dev bool
	var configPath []string
	var servers string
	var meta []string

	// Make a new, empty config.
	cmdConfig := &Config{
		Atlas:  &AtlasConfig{},
		Client: &ClientConfig{},
		Consul: &config.ConsulConfig{},
		Ports:  &Ports{},
		Server: &ServerConfig{},
		Vault:  &config.VaultConfig{},
		ACL:    &ACLConfig{},
	}

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Error(c.Help()) }

	// Role options
	flags.BoolVar(&dev, "dev", false, "")
	flags.BoolVar(&cmdConfig.Server.Enabled, "server", false, "")
	flags.BoolVar(&cmdConfig.Client.Enabled, "client", false, "")

	// Server-only options
	flags.IntVar(&cmdConfig.Server.BootstrapExpect, "bootstrap-expect", 0, "")
	flags.BoolVar(&cmdConfig.Server.RejoinAfterLeave, "rejoin", false, "")
	flags.Var((*flaghelper.StringFlag)(&cmdConfig.Server.StartJoin), "join", "")
	flags.Var((*flaghelper.StringFlag)(&cmdConfig.Server.RetryJoin), "retry-join", "")
	flags.IntVar(&cmdConfig.Server.RetryMaxAttempts, "retry-max", 0, "")
	flags.StringVar(&cmdConfig.Server.RetryInterval, "retry-interval", "", "")
	flags.StringVar(&cmdConfig.Server.EncryptKey, "encrypt", "", "gossip encryption key")

	// Client-only options
	flags.StringVar(&cmdConfig.Client.StateDir, "state-dir", "", "")
	flags.StringVar(&cmdConfig.Client.AllocDir, "alloc-dir", "", "")
	flags.StringVar(&cmdConfig.Client.NodeClass, "node-class", "", "")
	flags.StringVar(&servers, "servers", "", "")
	flags.Var((*flaghelper.StringFlag)(&meta), "meta", "")
	flags.StringVar(&cmdConfig.Client.NetworkInterface, "network-interface", "", "")
	flags.IntVar(&cmdConfig.Client.NetworkSpeed, "network-speed", 0, "")

	// General options
	flags.Var((*flaghelper.StringFlag)(&configPath), "config", "config")
	flags.StringVar(&cmdConfig.BindAddr, "bind", "", "")
	flags.StringVar(&cmdConfig.Region, "region", "", "")
	flags.StringVar(&cmdConfig.DataDir, "data-dir", "", "")
	flags.StringVar(&cmdConfig.Datacenter, "dc", "", "")
	flags.StringVar(&cmdConfig.LogLevel, "log-level", "", "")
	flags.StringVar(&cmdConfig.NodeName, "node", "", "")

	// Atlas options
	flags.StringVar(&cmdConfig.Atlas.Infrastructure, "atlas", "", "")
	flags.BoolVar(&cmdConfig.Atlas.Join, "atlas-join", false, "")
	flags.StringVar(&cmdConfig.Atlas.Token, "atlas-token", "", "")

	// Consul options
	flags.StringVar(&cmdConfig.Consul.Auth, "consul-auth", "", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.AutoAdvertise = &b
		return nil
	}), "consul-auto-advertise", "")
	flags.StringVar(&cmdConfig.Consul.CAFile, "consul-ca-file", "", "")
	flags.StringVar(&cmdConfig.Consul.CertFile, "consul-cert-file", "", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.ChecksUseAdvertise = &b
		return nil
	}), "consul-checks-use-advertise", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.ClientAutoJoin = &b
		return nil
	}), "consul-client-auto-join", "")
	flags.StringVar(&cmdConfig.Consul.ClientServiceName, "consul-client-service-name", "", "")
	flags.StringVar(&cmdConfig.Consul.KeyFile, "consul-key-file", "", "")
	flags.StringVar(&cmdConfig.Consul.ServerServiceName, "consul-server-service-name", "", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.ServerAutoJoin = &b
		return nil
	}), "consul-server-auto-join", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.EnableSSL = &b
		return nil
	}), "consul-ssl", "")
	flags.StringVar(&cmdConfig.Consul.Token, "consul-token", "", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Consul.VerifySSL = &b
		return nil
	}), "consul-verify-ssl", "")

	// Vault options
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Vault.Enabled = &b
		return nil
	}), "vault-enabled", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Vault.AllowUnauthenticated = &b
		return nil
	}), "vault-allow-unauthenticated", "")
	flags.StringVar(&cmdConfig.Vault.Token, "vault-token", "", "")
	flags.StringVar(&cmdConfig.Vault.Addr, "vault-address", "", "")
	flags.StringVar(&cmdConfig.Vault.Role, "vault-create-from-role", "", "")
	flags.StringVar(&cmdConfig.Vault.TLSCaFile, "vault-ca-file", "", "")
	flags.StringVar(&cmdConfig.Vault.TLSCaPath, "vault-ca-path", "", "")
	flags.StringVar(&cmdConfig.Vault.TLSCertFile, "vault-cert-file", "", "")
	flags.StringVar(&cmdConfig.Vault.TLSKeyFile, "vault-key-file", "", "")
	flags.Var((flaghelper.FuncBoolVar)(func(b bool) error {
		cmdConfig.Vault.TLSSkipVerify = &b
		return nil
	}), "vault-tls-skip-verify", "")
	flags.StringVar(&cmdConfig.Vault.TLSServerName, "vault-tls-server-name", "", "")

	// ACL options
	flags.BoolVar(&cmdConfig.ACL.Enabled, "acl-enabled", false, "")
	flags.StringVar(&cmdConfig.ACL.ReplicationToken, "acl-replication-token", "", "")

	if err := flags.Parse(c.args); err != nil {
		return nil
	}

	// Split the servers.
	if servers != "" {
		cmdConfig.Client.Servers = strings.Split(servers, ",")
	}

	// Parse the meta flags.
	metaLength := len(meta)
	if metaLength != 0 {
		cmdConfig.Client.Meta = make(map[string]string, metaLength)
		for _, kv := range meta {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				c.Ui.Error(fmt.Sprintf("Error parsing Client.Meta value: %v", kv))
				return nil
			}

			cmdConfig.Client.Meta[parts[0]] = parts[1]
		}
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

		// The user asked us to load some config here but we didn't find any,
		// so we'll complain but continue.
		if current == nil || reflect.DeepEqual(current, &Config{}) {
			c.Ui.Warn(fmt.Sprintf("No configuration loaded from %s", path))
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
	config.Version = c.Version

	// Normalize binds, ports, addresses, and advertise
	if err := config.normalizeAddrs(); err != nil {
		c.Ui.Error(err.Error())
		return nil
	}

	// Check to see if we should read the Vault token from the environment
	if config.Vault.Token == "" {
		if token, ok := os.LookupEnv("VAULT_TOKEN"); ok {
			config.Vault.Token = token
		}
	}

	if dev {
		// Skip validation for dev mode
		return config
	}

	if config.Server.EncryptKey != "" {
		if _, err := config.Server.EncryptBytes(); err != nil {
			c.Ui.Error(fmt.Sprintf("Invalid encryption key: %s", err))
			return nil
		}
		keyfile := filepath.Join(config.DataDir, serfKeyring)
		if _, err := os.Stat(keyfile); err == nil {
			c.Ui.Warn("WARNING: keyring exists but -encrypt given, using keyring")
		}
	}

	// Parse the RetryInterval.
	dur, err := time.ParseDuration(config.Server.RetryInterval)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing retry interval: %s", err))
		return nil
	}
	config.Server.retryInterval = dur

	// Check that the server is running in at least one mode.
	if !(config.Server.Enabled || config.Client.Enabled) {
		c.Ui.Error("Must specify either server, client or dev mode for the agent.")
		return nil
	}

	// Verify the paths are absolute.
	dirs := map[string]string{
		"data-dir":  config.DataDir,
		"alloc-dir": config.Client.AllocDir,
		"state-dir": config.Client.StateDir,
	}
	for k, dir := range dirs {
		if dir == "" {
			continue
		}

		if !filepath.IsAbs(dir) {
			c.Ui.Error(fmt.Sprintf("%s must be given as an absolute path: got %v", k, dir))
			return nil
		}
	}

	// Ensure that we have the directories we neet to run.
	if config.Server.Enabled && config.DataDir == "" {
		c.Ui.Error("Must specify data directory")
		return nil
	}

	// The config is valid if the top-level data-dir is set or if both
	// alloc-dir and state-dir are set.
	if config.Client.Enabled && config.DataDir == "" {
		if config.Client.AllocDir == "" || config.Client.StateDir == "" {
			c.Ui.Error("Must specify both the state and alloc dir if data-dir is omitted.")
			return nil
		}
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
	log.SetOutput(logOutput)
	return logGate, logWriter, logOutput
}

// setupAgent is used to start the agent and various interfaces
func (c *Command) setupAgent(config *Config, logOutput io.Writer, inmem *metrics.InmemSink) error {
	c.Ui.Output("Starting Nomad agent...")
	agent, err := NewAgent(config, logOutput, inmem)
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
	http, err := NewHTTPServer(agent, config)
	if err != nil {
		agent.Shutdown()
		c.Ui.Error(fmt.Sprintf("Error starting http server: %s", err))
		return err
	}
	c.httpServer = http

	// Setup update checking
	if !config.DisableUpdateCheck {
		version := config.Version.Version
		if config.Version.VersionPrerelease != "" {
			version += fmt.Sprintf("-%s", config.Version.VersionPrerelease)
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
			time.Sleep(lib.RandomStagger(30 * time.Second))
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
		c.Ui.Error(fmt.Sprintf("Newer Nomad version available: %s (currently running: %s)", results.CurrentVersion, c.Version.VersionNumber()))
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

func (c *Command) AutocompleteFlags() complete.Flags {
	configFilePredictor := complete.PredictOr(
		complete.PredictFiles("*.json"),
		complete.PredictFiles("*.hcl"))

	return map[string]complete.Predictor{
		"-config": configFilePredictor,
	}
}

func (c *Command) AutocompleteArgs() complete.Predictor {
	return nil
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

	// Log config files
	if len(config.Files) > 0 {
		c.Ui.Info(fmt.Sprintf("Loaded configuration from %s", strings.Join(config.Files, ", ")))
	} else {
		c.Ui.Info("No configuration files loaded")
	}

	// Initialize the telemetry
	inmem, err := c.setupTelemetry(config)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing telemetry: %s", err))
		return 1
	}

	// Create the agent
	if err := c.setupAgent(config, logOutput, inmem); err != nil {
		logGate.Flush()
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

	// Join startup nodes if specified
	if err := c.startupJoin(config); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Compile agent information for output later
	info := make(map[string]string)
	info["version"] = config.Version.VersionNumber()
	info["client"] = strconv.FormatBool(config.Client.Enabled)
	info["log level"] = config.LogLevel
	info["server"] = strconv.FormatBool(config.Server.Enabled)
	info["region"] = fmt.Sprintf("%s (DC: %s)", config.Region, config.Datacenter)

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

	// Start retry join process
	c.retryJoinErrCh = make(chan struct{})
	go c.retryJoin(config)

	// Wait for exit
	return c.handleSignals(config)
}

// handleSignals blocks until we get an exit-causing signal
func (c *Command) handleSignals(config *Config) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	// Wait for a signal
WAIT:
	var sig os.Signal
	select {
	case s := <-signalCh:
		sig = s
	case <-c.ShutdownCh:
		sig = os.Interrupt
	case <-c.retryJoinErrCh:
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Caught signal: %v", sig))

	// Skip any SIGPIPE signal (See issue #1798)
	if sig == syscall.SIGPIPE {
		goto WAIT
	}

	// Check if this is a SIGHUP
	if sig == syscall.SIGHUP {
		if conf := c.handleReload(config); conf != nil {
			*config = *conf
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

	if s := c.agent.Server(); s != nil {
		sconf, err := convertServerConfig(newConf, c.logOutput)
		if err != nil {
			c.agent.logger.Printf("[ERR] agent: failed to convert server config: %v", err)
		} else {
			if err := s.Reload(sconf); err != nil {
				c.agent.logger.Printf("[ERR] agent: reloading server config failed: %v", err)
			}
		}
	}

	return newConf
}

// setupTelemetry is used ot setup the telemetry sub-systems
func (c *Command) setupTelemetry(config *Config) (*metrics.InmemSink, error) {
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
	if telConfig.UseNodeName {
		metricsConf.HostName = config.NodeName
		metricsConf.EnableHostname = true
	}

	// Configure the statsite sink
	var fanout metrics.FanoutSink
	if telConfig.StatsiteAddr != "" {
		sink, err := metrics.NewStatsiteSink(telConfig.StatsiteAddr)
		if err != nil {
			return inm, err
		}
		fanout = append(fanout, sink)
	}

	// Configure the statsd sink
	if telConfig.StatsdAddr != "" {
		sink, err := metrics.NewStatsdSink(telConfig.StatsdAddr)
		if err != nil {
			return inm, err
		}
		fanout = append(fanout, sink)
	}

	// Configure the prometheus sink
	if telConfig.PrometheusMetrics {
		promSink, err := prometheus.NewPrometheusSink()
		if err != nil {
			return inm, err
		}
		fanout = append(fanout, promSink)
	}

	// Configure the datadog sink
	if telConfig.DataDogAddr != "" {
		sink, err := datadog.NewDogStatsdSink(telConfig.DataDogAddr, config.NodeName)
		if err != nil {
			return inm, err
		}
		fanout = append(fanout, sink)
	}

	// Configure the Circonus sink
	if telConfig.CirconusAPIToken != "" || telConfig.CirconusCheckSubmissionURL != "" {
		cfg := &circonus.Config{}
		cfg.Interval = telConfig.CirconusSubmissionInterval
		cfg.CheckManager.API.TokenKey = telConfig.CirconusAPIToken
		cfg.CheckManager.API.TokenApp = telConfig.CirconusAPIApp
		cfg.CheckManager.API.URL = telConfig.CirconusAPIURL
		cfg.CheckManager.Check.SubmissionURL = telConfig.CirconusCheckSubmissionURL
		cfg.CheckManager.Check.ID = telConfig.CirconusCheckID
		cfg.CheckManager.Check.ForceMetricActivation = telConfig.CirconusCheckForceMetricActivation
		cfg.CheckManager.Check.InstanceID = telConfig.CirconusCheckInstanceID
		cfg.CheckManager.Check.SearchTag = telConfig.CirconusCheckSearchTag
		cfg.CheckManager.Check.Tags = telConfig.CirconusCheckTags
		cfg.CheckManager.Check.DisplayName = telConfig.CirconusCheckDisplayName
		cfg.CheckManager.Broker.ID = telConfig.CirconusBrokerID
		cfg.CheckManager.Broker.SelectTag = telConfig.CirconusBrokerSelectTag

		if cfg.CheckManager.Check.DisplayName == "" {
			cfg.CheckManager.Check.DisplayName = "Nomad"
		}

		if cfg.CheckManager.API.TokenApp == "" {
			cfg.CheckManager.API.TokenApp = "nomad"
		}

		if cfg.CheckManager.Check.SearchTag == "" {
			cfg.CheckManager.Check.SearchTag = "service:nomad"
		}

		sink, err := circonus.NewCirconusSink(cfg)
		if err != nil {
			return inm, err
		}
		sink.Start()
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
	return inm, nil
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

	scadaConfig := &scada.Config{
		Service:      "nomad",
		Version:      config.Version.VersionNumber(),
		ResourceType: "nomad-cluster",
		Meta: map[string]string{
			"auto-join":  strconv.FormatBool(config.Atlas.Join),
			"region":     config.Region,
			"datacenter": config.Datacenter,
			"client":     strconv.FormatBool(config.Client != nil && config.Client.Enabled),
			"server":     strconv.FormatBool(config.Server != nil && config.Server.Enabled),
		},
		Atlas: scada.AtlasConfig{
			Endpoint:       config.Atlas.Endpoint,
			Infrastructure: config.Atlas.Infrastructure,
			Token:          config.Atlas.Token,
		},
	}

	provider, list, err := scada.NewHTTPProvider(scadaConfig, c.logOutput)
	if err != nil {
		return err
	}
	c.scadaProvider = provider
	c.scadaHttp = newScadaHttp(c.agent, list)
	return nil
}

func (c *Command) startupJoin(config *Config) error {
	if len(config.Server.StartJoin) == 0 || !config.Server.Enabled {
		return nil
	}

	c.Ui.Output("Joining cluster...")
	n, err := c.agent.server.Join(config.Server.StartJoin)
	if err != nil {
		return err
	}

	c.Ui.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
	return nil
}

// retryJoin is used to handle retrying a join until it succeeds or all retries
// are exhausted.
func (c *Command) retryJoin(config *Config) {
	if len(config.Server.RetryJoin) == 0 || !config.Server.Enabled {
		return
	}

	logger := c.agent.logger
	logger.Printf("[INFO] agent: Joining cluster...")

	attempt := 0
	for {
		n, err := c.agent.server.Join(config.Server.RetryJoin)
		if err == nil {
			logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
			return
		}

		attempt++
		if config.Server.RetryMaxAttempts > 0 && attempt > config.Server.RetryMaxAttempts {
			logger.Printf("[ERR] agent: max join retry exhausted, exiting")
			close(c.retryJoinErrCh)
			return
		}

		logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err,
			config.Server.RetryInterval)
		time.Sleep(config.Server.retryInterval)
	}
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

  -encrypt=<key>
    Provides the gossip encryption key

  -join=<address>
    Address of an agent to join at start time. Can be specified
    multiple times.

  -retry-join=<address>
    Address of an agent to join at start time with retries enabled.
    Can be specified multiple times.

  -retry-max=<num>
    Maximum number of join attempts. Defaults to 0, which will retry
    indefinitely.

  -retry-interval=<dur>
    Time to wait between join attempts.

  -rejoin
    Ignore a previous leave and attempts to rejoin the cluster.

Client Options:

  -client
    Enable client mode for the agent. Client mode enables a given node to be
    evaluated for allocations. If client mode is not enabled, no work will be
    scheduled to the agent.

  -state-dir
    The directory used to store state and other persistent data. If not
    specified a subdirectory under the "-data-dir" will be used.

  -alloc-dir
    The directory used to store allocation data such as downloaded artificats as
    well as data produced by tasks. If not specified, a subdirectory under the
    "-data-dir" will be used.

  -servers
    A list of known server addresses to connect to given as "host:port" and
    delimited by commas.

  -node-class
    Mark this node as a member of a node-class. This can be used to label
    similar node types.

  -meta
    User specified metadata to associated with the node. Each instance of -meta
    parses a single KEY=VALUE pair. Repeat the meta flag for each key/value pair
    to be added.

  -network-interface
    Forces the network fingerprinter to use the specified network interface.

  -network-speed
    The default speed for network interfaces in MBits if the link speed can not
    be determined dynamically.

ACL Options:

  -acl-enabled
    Specifies whether the agent should enable ACLs.

  -acl-replication-token
    The replication token for servers to use when replicating from the
    authoratative region. The token must be a valid management token from the
    authoratative region.

Consul Options:

  -consul-address=<addr>
    Specifies the address to the local Consul agent, given in the format host:port.
    Supports Unix sockets with the format: unix:///tmp/consul/consul.sock

  -consul-auth=<auth>
    Specifies the HTTP Basic Authentication information to use for access to the
    Consul Agent, given in the format username:password.

  -consul-auto-advertise
    Specifies if Nomad should advertise its services in Consul. The services
    are named according to server_service_name and client_service_name. Nomad
    servers and clients advertise their respective services, each tagged
    appropriately with either http or rpc tag. Nomad servers also advertise a
    serf tagged service.

  -consul-ca-file=<path>
    Specifies an optional path to the CA certificate used for Consul communication.
    This defaults to the system bundle if unspecified.

  -consul-cert-file=<path>
    Specifies the path to the certificate used for Consul communication. If this
    is set then you need to also set key_file.

  -consul-checks-use-advertise
    Specifies if Consul heath checks should bind to the advertise address. By
    default, this is the bind address.

  -consul-client-auto-join
    Specifies if the Nomad clients should automatically discover servers in the
    same region by searching for the Consul service name defined in the
    server_service_name option.

  -consul-client-service-name=<name>
    Specifies the name of the service in Consul for the Nomad clients.

  -consul-key-file=<path>
    Specifies the path to the private key used for Consul communication. If this
    is set then you need to also set cert_file.

  -consul-server-service-name=<name>
    Specifies the name of the service in Consul for the Nomad servers.

  -consul-server-auto-join
    Specifies if the Nomad servers should automatically discover and join other
    Nomad servers by searching for the Consul service name defined in the
    server_service_name option. This search only happens if the server does not
    have a leader.

  -consul-ssl
    Specifies if the transport scheme should use HTTPS to communicate with the
    Consul agent.

  -consul-token=<token>
    Specifies the token used to provide a per-request ACL token.

  -consul-verify-ssl
    Specifies if SSL peer verification should be used when communicating to the
    Consul API client over HTTPS.

Vault Options:

  -vault-enabled
    Whether to enable or disable Vault integration.

  -vault-address=<addr>
    The address to communicate with Vault. This should be provided with the http://
    or https:// prefix.

  -vault-token=<token>
    The Vault token used to derive tokens from Vault on behalf of clients.
    This only needs to be set on Servers. Overrides the Vault token read from
    the VAULT_TOKEN environment variable.

  -vault-create-from-role=<role>
    The role name to create tokens for tasks from.

  -vault-allow-unauthenticated
    Whether to allow jobs to be sumbitted that request Vault Tokens but do not
    authentication. The flag only applies to Servers.

  -vault-ca-file=<path>
    The path to a PEM-encoded CA cert file to use to verify the Vault server SSL
    certificate.

  -vault-ca-path=<path>
    The path to a directory of PEM-encoded CA cert files to verify the Vault server
    certificate.

  -vault-cert-file=<token>
    The path to the certificate for Vault communication.

  -vault-key-file=<addr>
    The path to the private key for Vault communication.

  -vault-tls-skip-verify=<token>
    Enables or disables SSL certificate verification.

  -vault-tls-server-name=<token>
    Used to set the SNI host when connecting over TLS.

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
