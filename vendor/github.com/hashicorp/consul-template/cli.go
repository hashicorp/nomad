package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/hashicorp/consul-template/config"
	"github.com/hashicorp/consul-template/logging"
	"github.com/hashicorp/consul-template/manager"
	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/consul-template/version"
)

// Exit codes are int values that represent an exit code for a particular error.
// Sub-systems may check this unique error to determine the cause of an error
// without parsing the output or help text.
//
// Errors start at 10
const (
	ExitCodeOK int = 0

	ExitCodeError = 10 + iota
	ExitCodeInterrupt
	ExitCodeParseFlagsError
	ExitCodeRunnerError
	ExitCodeConfigError
)

// CLI is the main entry point.
type CLI struct {
	sync.Mutex

	// outSteam and errStream are the standard out and standard error streams to
	// write messages from the CLI.
	outStream, errStream io.Writer

	// signalCh is the channel where the cli receives signals.
	signalCh chan os.Signal

	// stopCh is an internal channel used to trigger a shutdown of the CLI.
	stopCh  chan struct{}
	stopped bool
}

// NewCLI creates a new CLI object with the given stdout and stderr streams.
func NewCLI(out, err io.Writer) *CLI {
	return &CLI{
		outStream: out,
		errStream: err,
		signalCh:  make(chan os.Signal, 1),
		stopCh:    make(chan struct{}),
	}
}

// Run accepts a slice of arguments and returns an int representing the exit
// status from the command.
func (cli *CLI) Run(args []string) int {
	// Parse the flags
	config, paths, once, dry, isVersion, err := cli.ParseFlags(args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintf(cli.errStream, usage, version.Name)
			return 0
		}
		fmt.Fprintln(cli.errStream, err.Error())
		return ExitCodeParseFlagsError
	}

	// Save original config (defaults + parsed flags) for handling reloads
	cliConfig := config.Copy()

	// Load configuration paths, with CLI taking precedence
	config, err = loadConfigs(paths, cliConfig)
	if err != nil {
		return logError(err, ExitCodeConfigError)
	}

	config.Finalize()

	// Setup the config and logging
	config, err = cli.setup(config)
	if err != nil {
		return logError(err, ExitCodeConfigError)
	}

	// Print version information for debugging
	log.Printf("[INFO] %s", version.HumanVersion)

	// If the version was requested, return an "error" containing the version
	// information. This might sound weird, but most *nix applications actually
	// print their version on stderr anyway.
	if isVersion {
		log.Printf("[DEBUG] (cli) version flag was given, exiting now")
		fmt.Fprintf(cli.errStream, "%s\n", version.HumanVersion)
		return ExitCodeOK
	}

	// Initial runner
	runner, err := manager.NewRunner(config, dry, once)
	if err != nil {
		return logError(err, ExitCodeRunnerError)
	}
	go runner.Start()

	// Listen for signals
	signal.Notify(cli.signalCh)

	for {
		select {
		case err := <-runner.ErrCh:
			// Check if the runner's error returned a specific exit status, and return
			// that value. If no value was given, return a generic exit status.
			code := ExitCodeRunnerError
			if typed, ok := err.(manager.ErrExitable); ok {
				code = typed.ExitStatus()
			}
			return logError(err, code)
		case <-runner.DoneCh:
			return ExitCodeOK
		case s := <-cli.signalCh:
			log.Printf("[DEBUG] (cli) receiving signal %q", s)

			switch s {
			case *config.ReloadSignal:
				fmt.Fprintf(cli.errStream, "Reloading configuration...\n")
				runner.Stop()

				// Re-parse any configuration files or paths
				config, err = loadConfigs(paths, cliConfig)
				if err != nil {
					return logError(err, ExitCodeConfigError)
				}
				config.Finalize()

				// Load the new configuration from disk
				config, err = cli.setup(config)
				if err != nil {
					return logError(err, ExitCodeConfigError)
				}

				runner, err = manager.NewRunner(config, dry, once)
				if err != nil {
					return logError(err, ExitCodeRunnerError)
				}
				go runner.Start()
			case *config.KillSignal:
				fmt.Fprintf(cli.errStream, "Cleaning up...\n")
				runner.Stop()
				return ExitCodeInterrupt
			case signals.SignalLookup["SIGCHLD"]:
				// The SIGCHLD signal is sent to the parent of a child process when it
				// exits, is interrupted, or resumes after being interrupted. We ignore
				// this signal because the child process is monitored on its own.
				//
				// Also, the reason we do a lookup instead of a direct syscall.SIGCHLD
				// is because that isn't defined on Windows.
			default:
				// Propagate the signal to the child process
				runner.Signal(s)
			}
		case <-cli.stopCh:
			return ExitCodeOK
		}
	}
}

// stop is used internally to shutdown a running CLI
func (cli *CLI) stop() {
	cli.Lock()
	defer cli.Unlock()

	if cli.stopped {
		return
	}

	close(cli.stopCh)
	cli.stopped = true
}

// ParseFlags is a helper function for parsing command line flags using Go's
// Flag library. This is extracted into a helper to keep the main function
// small, but it also makes writing tests for parsing command line arguments
// much easier and cleaner.
func (cli *CLI) ParseFlags(args []string) (*config.Config, []string, bool, bool, bool, error) {
	var dry, once, isVersion bool

	c := config.DefaultConfig()

	// configPaths stores the list of configuration paths on disk
	configPaths := make([]string, 0, 6)

	// Parse the flags and options
	flags := flag.NewFlagSet(version.Name, flag.ContinueOnError)
	flags.SetOutput(ioutil.Discard)
	flags.Usage = func() {}

	flags.Var((funcVar)(func(s string) error {
		configPaths = append(configPaths, s)
		return nil
	}), "config", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.Address = config.String(s)
		return nil
	}), "consul-addr", "")

	flags.Var((funcVar)(func(s string) error {
		a, err := config.ParseAuthConfig(s)
		if err != nil {
			return err
		}
		c.Consul.Auth = a
		return nil
	}), "consul-auth", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Consul.Retry.Enabled = config.Bool(b)
		return nil
	}), "consul-retry", "")

	flags.Var((funcIntVar)(func(i int) error {
		c.Consul.Retry.Attempts = config.Int(i)
		return nil
	}), "consul-retry-attempts", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Retry.Backoff = config.TimeDuration(d)
		return nil
	}), "consul-retry-backoff", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Retry.MaxBackoff = config.TimeDuration(d)
		return nil
	}), "consul-retry-max-backoff", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Consul.SSL.Enabled = config.Bool(b)
		return nil
	}), "consul-ssl", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.SSL.CaCert = config.String(s)
		return nil
	}), "consul-ssl-ca-cert", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.SSL.CaPath = config.String(s)
		return nil
	}), "consul-ssl-ca-path", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.SSL.Cert = config.String(s)
		return nil
	}), "consul-ssl-cert", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.SSL.Key = config.String(s)
		return nil
	}), "consul-ssl-key", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.SSL.ServerName = config.String(s)
		return nil
	}), "consul-ssl-server-name", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Consul.SSL.Verify = config.Bool(b)
		return nil
	}), "consul-ssl-verify", "")

	flags.Var((funcVar)(func(s string) error {
		c.Consul.Token = config.String(s)
		return nil
	}), "consul-token", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Transport.DialKeepAlive = config.TimeDuration(d)
		return nil
	}), "consul-transport-dial-keep-alive", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Transport.DialTimeout = config.TimeDuration(d)
		return nil
	}), "consul-transport-dial-timeout", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Consul.Transport.DisableKeepAlives = config.Bool(b)
		return nil
	}), "consul-transport-disable-keep-alives", "")

	flags.Var((funcIntVar)(func(i int) error {
		c.Consul.Transport.MaxIdleConnsPerHost = config.Int(i)
		return nil
	}), "consul-transport-max-idle-conns-per-host", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Transport.TLSHandshakeTimeout = config.TimeDuration(d)
		return nil
	}), "consul-transport-tls-handshake-timeout", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Dedup.Enabled = config.Bool(b)
		return nil
	}), "dedup", "")

	flags.BoolVar(&dry, "dry", false, "")

	flags.Var((funcVar)(func(s string) error {
		c.Exec.Enabled = config.Bool(true)
		c.Exec.Command = config.String(s)
		return nil
	}), "exec", "")

	flags.Var((funcVar)(func(s string) error {
		sig, err := signals.Parse(s)
		if err != nil {
			return err
		}
		c.Exec.KillSignal = config.Signal(sig)
		return nil
	}), "exec-kill-signal", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Exec.KillTimeout = config.TimeDuration(d)
		return nil
	}), "exec-kill-timeout", "")

	flags.Var((funcVar)(func(s string) error {
		sig, err := signals.Parse(s)
		if err != nil {
			return err
		}
		c.Exec.ReloadSignal = config.Signal(sig)
		return nil
	}), "exec-reload-signal", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Exec.Splay = config.TimeDuration(d)
		return nil
	}), "exec-splay", "")

	flags.Var((funcVar)(func(s string) error {
		sig, err := signals.Parse(s)
		if err != nil {
			return err
		}
		c.KillSignal = config.Signal(sig)
		return nil
	}), "kill-signal", "")

	flags.Var((funcVar)(func(s string) error {
		c.LogLevel = config.String(s)
		return nil
	}), "log-level", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.MaxStale = config.TimeDuration(d)
		return nil
	}), "max-stale", "")

	flags.BoolVar(&once, "once", false, "")

	flags.Var((funcVar)(func(s string) error {
		c.PidFile = config.String(s)
		return nil
	}), "pid-file", "")

	flags.Var((funcVar)(func(s string) error {
		sig, err := signals.Parse(s)
		if err != nil {
			return err
		}
		c.ReloadSignal = config.Signal(sig)
		return nil
	}), "reload-signal", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Consul.Retry.Backoff = config.TimeDuration(d)
		return nil
	}), "retry", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Syslog.Enabled = config.Bool(b)
		return nil
	}), "syslog", "")

	flags.Var((funcVar)(func(s string) error {
		c.Syslog.Facility = config.String(s)
		return nil
	}), "syslog-facility", "")

	flags.Var((funcVar)(func(s string) error {
		t, err := config.ParseTemplateConfig(s)
		if err != nil {
			return err
		}
		*c.Templates = append(*c.Templates, t)
		return nil
	}), "template", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.Address = config.String(s)
		return nil
	}), "vault-addr", "")

	flags.Var((funcDurationVar)(func(t time.Duration) error {
		c.Vault.Grace = config.TimeDuration(t)
		return nil
	}), "vault-grace", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.RenewToken = config.Bool(b)
		return nil
	}), "vault-renew-token", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.Retry.Enabled = config.Bool(b)
		return nil
	}), "vault-retry", "")

	flags.Var((funcIntVar)(func(i int) error {
		c.Vault.Retry.Attempts = config.Int(i)
		return nil
	}), "vault-retry-attempts", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Vault.Retry.Backoff = config.TimeDuration(d)
		return nil
	}), "vault-retry-backoff", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Vault.Retry.MaxBackoff = config.TimeDuration(d)
		return nil
	}), "vault-retry-max-backoff", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.SSL.Enabled = config.Bool(b)
		return nil
	}), "vault-ssl", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.SSL.CaCert = config.String(s)
		return nil
	}), "vault-ssl-ca-cert", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.SSL.CaPath = config.String(s)
		return nil
	}), "vault-ssl-ca-path", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.SSL.Cert = config.String(s)
		return nil
	}), "vault-ssl-cert", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.SSL.Key = config.String(s)
		return nil
	}), "vault-ssl-key", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.SSL.ServerName = config.String(s)
		return nil
	}), "vault-ssl-server-name", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.SSL.Verify = config.Bool(b)
		return nil
	}), "vault-ssl-verify", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Vault.Transport.DialKeepAlive = config.TimeDuration(d)
		return nil
	}), "vault-transport-dial-keep-alive", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Vault.Transport.DialTimeout = config.TimeDuration(d)
		return nil
	}), "vault-transport-dial-timeout", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.Transport.DisableKeepAlives = config.Bool(b)
		return nil
	}), "vault-transport-disable-keep-alives", "")

	flags.Var((funcIntVar)(func(i int) error {
		c.Vault.Transport.MaxIdleConnsPerHost = config.Int(i)
		return nil
	}), "vault-transport-max-idle-conns-per-host", "")

	flags.Var((funcDurationVar)(func(d time.Duration) error {
		c.Vault.Transport.TLSHandshakeTimeout = config.TimeDuration(d)
		return nil
	}), "vault-transport-tls-handshake-timeout", "")

	flags.Var((funcVar)(func(s string) error {
		c.Vault.Token = config.String(s)
		return nil
	}), "vault-token", "")

	flags.Var((funcBoolVar)(func(b bool) error {
		c.Vault.UnwrapToken = config.Bool(b)
		return nil
	}), "vault-unwrap-token", "")

	flags.Var((funcVar)(func(s string) error {
		w, err := config.ParseWaitConfig(s)
		if err != nil {
			return err
		}
		c.Wait = w
		return nil
	}), "wait", "")

	flags.BoolVar(&isVersion, "v", false, "")
	flags.BoolVar(&isVersion, "version", false, "")

	// If there was a parser error, stop
	if err := flags.Parse(args); err != nil {
		return nil, nil, false, false, false, err
	}

	// Error if extra arguments are present
	args = flags.Args()
	if len(args) > 0 {
		return nil, nil, false, false, false, fmt.Errorf("cli: extra args: %q", args)
	}

	return c, configPaths, once, dry, isVersion, nil
}

// loadConfigs loads the configuration from the list of paths. The optional
// configuration is the list of overrides to apply at the very end, taking
// precedence over any configurations that were loaded from the paths. If any
// errors occur when reading or parsing those sub-configs, it is returned.
func loadConfigs(paths []string, o *config.Config) (*config.Config, error) {
	finalC := config.DefaultConfig()

	for _, path := range paths {
		c, err := config.FromPath(path)
		if err != nil {
			return nil, err
		}

		finalC = finalC.Merge(c)
	}

	finalC = finalC.Merge(o)
	finalC.Finalize()
	return finalC, nil
}

// logError logs an error message and then returns the given status.
func logError(err error, status int) int {
	log.Printf("[ERR] (cli) %s", err)
	return status
}

func (cli *CLI) setup(conf *config.Config) (*config.Config, error) {
	if err := logging.Setup(&logging.Config{
		Name:           version.Name,
		Level:          config.StringVal(conf.LogLevel),
		Syslog:         config.BoolVal(conf.Syslog.Enabled),
		SyslogFacility: config.StringVal(conf.Syslog.Facility),
		Writer:         cli.errStream,
	}); err != nil {
		return nil, err
	}

	return conf, nil
}

const usage = `Usage: %s [options]

  Watches a series of templates on the file system, writing new changes when
  Consul is updated. It runs until an interrupt is received unless the -once
  flag is specified.

Options:

  -config=<path>
      Sets the path to a configuration file or folder on disk. This can be
      specified multiple times to load multiple files or folders. If multiple
      values are given, they are merged left-to-right, and CLI arguments take
      the top-most precedence.

  -consul-addr=<address>
      Sets the address of the Consul instance

  -consul-auth=<username[:password]>
      Set the basic authentication username and password for communicating
      with Consul.

  -consul-retry
      Use retry logic when communication with Consul fails

  -consul-retry-attempts=<int>
      The number of attempts to use when retrying failed communications

  -consul-retry-backoff=<duration>
      The base amount to use for the backoff duration. This number will be
      increased exponentially for each retry attempt.

  -consul-retry-max-backoff=<duration>
      The maximum limit of the retry backoff duration. Default is one minute.
      0 means infinite. The backoff will increase exponentially until given value.

  -consul-ssl
      Use SSL when connecting to Consul

  -consul-ssl-ca-cert=<string>
      Validate server certificate against this CA certificate file list

  -consul-ssl-ca-path=<string>
      Sets the path to the CA to use for TLS verification

  -consul-ssl-cert=<string>
      SSL client certificate to send to server

  -consul-ssl-key=<string>
      SSL/TLS private key for use in client authentication key exchange

  -consul-ssl-server-name=<string>
      Sets the name of the server to use when validating TLS.

  -consul-ssl-verify
      Verify certificates when connecting via SSL

  -consul-token=<token>
      Sets the Consul API token

  -consul-transport-dial-keep-alive=<duration>
      Sets the amount of time to use for keep-alives

  -consul-transport-dial-timeout=<duration>
      Sets the amount of time to wait to establish a connection

  -consul-transport-disable-keep-alives
      Disables keep-alives (this will impact performance)

  -consul-transport-max-idle-conns-per-host=<int>
      Sets the maximum number of idle connections to permit per host

  -consul-transport-tls-handshake-timeout=<duration>
      Sets the handshake timeout

  -dedup
      Enable de-duplication mode - reduces load on Consul when many instances of
      Consul Template are rendering a common template

  -dry
      Print generated templates to stdout instead of rendering

  -exec=<command>
      Enable exec mode to run as a supervisor-like process - the given command
      will receive all signals provided to the parent process and will receive a
      signal when templates change

  -exec-kill-signal=<signal>
      Signal to send when gracefully killing the process

  -exec-kill-timeout=<duration>
      Amount of time to wait before force-killing the child

  -exec-reload-signal=<signal>
      Signal to send when a reload takes place

  -exec-splay=<duration>
      Amount of time to wait before sending signals

  -kill-signal=<signal>
      Signal to listen to gracefully terminate the process

  -log-level=<level>
      Set the logging level - values are "debug", "info", "warn", and "err"

  -max-stale=<duration>
      Set the maximum staleness and allow stale queries to Consul which will
      distribute work among all servers instead of just the leader

  -once
      Do not run the process as a daemon

  -pid-file=<path>
      Path on disk to write the PID of the process

  -reload-signal=<signal>
      Signal to listen to reload configuration

  -retry=<duration>
      The amount of time to wait if Consul returns an error when communicating
      with the API

  -syslog
      Send the output to syslog instead of standard error and standard out. The
      syslog facility defaults to LOCAL0 and can be changed using a
      configuration file

  -syslog-facility=<facility>
      Set the facility where syslog should log - if this attribute is supplied,
      the -syslog flag must also be supplied

  -template=<template>
       Adds a new template to watch on disk in the format 'in:out(:command)'

  -vault-addr=<address>
      Sets the address of the Vault server

  -vault-grace=<duration>
      Sets the grace period between lease renewal and secret re-acquisition - if
      the remaining lease duration is less than this value, Consul Template will
      acquire a new secret from Vault

  -vault-renew-token
      Periodically renew the provided Vault API token - this defaults to "true"
      and will renew the token at half of the lease duration

  -vault-retry
      Use retry logic when communication with Vault fails

  -vault-retry-attempts=<int>
      The number of attempts to use when retrying failed communications

  -vault-retry-backoff=<duration>
      The base amount to use for the backoff duration. This number will be
      increased exponentially for each retry attempt.

  -vault-retry-max-backoff=<duration>
      The maximum limit of the retry backoff duration. Default is one minute.
      0 means infinite. The backoff will increase exponentially until given value.

  -vault-ssl
      Specifies whether communications with Vault should be done via SSL

  -vault-ssl-ca-cert=<string>
      Sets the path to the CA certificate to use for TLS verification

  -vault-ssl-ca-path=<string>
      Sets the path to the CA to use for TLS verification

  -vault-ssl-cert=<string>
      Sets the path to the certificate to use for TLS verification

  -vault-ssl-key=<string>
      Sets the path to the key to use for TLS verification

  -vault-ssl-server-name=<string>
      Sets the name of the server to use when validating TLS.

  -vault-ssl-verify
      Enable SSL verification for communications with Vault.

  -vault-token=<token>
      Sets the Vault API token

  -vault-transport-dial-keep-alive=<duration>
      Sets the amount of time to use for keep-alives

  -vault-transport-dial-timeout=<duration>
      Sets the amount of time to wait to establish a connection

  -vault-transport-disable-keep-alives
      Disables keep-alives (this will impact performance)

  -vault-transport-max-idle-conns-per-host=<int>
      Sets the maximum number of idle connections to permit per host

  -vault-transport-tls-handshake-timeout=<duration>
      Sets the handshake timeout

  -vault-unwrap-token
      Unwrap the provided Vault API token (see Vault documentation for more
      information on this feature)

  -wait=<duration>
      Sets the 'min(:max)' amount of time to wait before writing a template (and
      triggering a command)

  -v, -version
      Print the version of this daemon
`
