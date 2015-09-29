package command

import (
	"bufio"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

const (
	// Names of environment variables used to supply various
	// config options to the Nomad CLI.
	EnvNomadAddress = "NOMAD_ADDR"
)

// FlagSetFlags is an enum to define what flags are present in the
// default FlagSet returned by Meta.FlagSet.
type FlagSetFlags uint

const (
	FlagSetNone    FlagSetFlags = 0
	FlagSetClient  FlagSetFlags = 1 << iota
	FlagSetDefault              = FlagSetClient
)

// Meta contains the meta-options and functionality that nearly every
// Nomad command inherits.
type Meta struct {
	Ui cli.Ui

	// These are set by the command line flags.
	flagAddress string
}

// FlagSet returns a FlagSet with the common flags that every
// command implements. The exact behavior of FlagSet can be configured
// using the flags as the second parameter, for example to disable
// server settings on the commands that don't talk to a server.
func (m *Meta) FlagSet(n string, fs FlagSetFlags) *flag.FlagSet {
	f := flag.NewFlagSet(n, flag.ContinueOnError)

	// FlagSetClient is used to enable the settings for specifying
	// client connectivity options.
	if fs&FlagSetClient != 0 {
		f.StringVar(&m.flagAddress, "address", "", "")
	}

	// Create an io.Writer that writes to our UI properly for errors.
	// This is kind of a hack, but it does the job. Basically: create
	// a pipe, use a scanner to break it into lines, and output each line
	// to the UI. Do this forever.
	errR, errW := io.Pipe()
	errScanner := bufio.NewScanner(errR)
	go func() {
		for errScanner.Scan() {
			m.Ui.Error(errScanner.Text())
		}
	}()
	f.SetOutput(errW)

	return f
}

// Client is used to initialize and return a new API client using
// the default command line arguments and env vars.
func (m *Meta) Client() (*api.Client, error) {
	config := api.DefaultConfig()
	if v := os.Getenv(EnvNomadAddress); v != "" {
		config.Address = v
	}
	if m.flagAddress != "" {
		config.Address = m.flagAddress
	}
	return api.NewClient(config)
}

// generalOptionsUsage returns the help string for the global options.
func generalOptionsUsage() string {
	helpText := `
  -address=<addr>
    The address of the Nomad server.
    Overrides the NOMAD_ADDR environment variable if set.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}
