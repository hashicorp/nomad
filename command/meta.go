package command

import (
	"bufio"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
	"github.com/posener/complete"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// Constants for CLI identifier length
	shortId = 8
	fullId  = 36
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

	// Whether to not-colorize output
	noColor bool

	// The region to send API requests
	region string

	// namespace to send API requests
	namespace string

	caCert     string
	caPath     string
	clientCert string
	clientKey  string
	insecure   bool
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
		f.StringVar(&m.region, "region", "", "")
		f.StringVar(&m.namespace, "namespace", "", "")
		f.BoolVar(&m.noColor, "no-color", false, "")
		f.StringVar(&m.caCert, "ca-cert", "", "")
		f.StringVar(&m.caPath, "ca-path", "", "")
		f.StringVar(&m.clientCert, "client-cert", "", "")
		f.StringVar(&m.clientKey, "client-key", "", "")
		f.BoolVar(&m.insecure, "insecure", false, "")
		f.BoolVar(&m.insecure, "tls-skip-verify", false, "")

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

// AutocompleteFlags returns a set of flag completions for the given flag set.
func (m *Meta) AutocompleteFlags(fs FlagSetFlags) complete.Flags {
	if fs&FlagSetClient == 0 {
		return nil
	}

	return complete.Flags{
		"-address":         complete.PredictAnything,
		"-region":          complete.PredictAnything,
		"-namespace":       NamespacePredictor(m.Client, nil),
		"-no-color":        complete.PredictNothing,
		"-ca-cert":         complete.PredictFiles("*"),
		"-ca-path":         complete.PredictDirs("*"),
		"-client-cert":     complete.PredictFiles("*"),
		"-client-key":      complete.PredictFiles("*"),
		"-insecure":        complete.PredictNothing,
		"-tls-skip-verify": complete.PredictNothing,
	}
}

// ApiClientFactory is the signature of a API client factory
type ApiClientFactory func() (*api.Client, error)

// Client is used to initialize and return a new API client using
// the default command line arguments and env vars.
func (m *Meta) Client() (*api.Client, error) {
	config := api.DefaultConfig()
	if m.flagAddress != "" {
		config.Address = m.flagAddress
	}
	if m.region != "" {
		config.Region = m.region
	}
	if m.namespace != "" {
		config.Namespace = m.namespace
	}

	// If we need custom TLS configuration, then set it
	if m.caCert != "" || m.caPath != "" || m.clientCert != "" || m.clientKey != "" || m.insecure {
		t := &api.TLSConfig{
			CACert:     m.caCert,
			CAPath:     m.caPath,
			ClientCert: m.clientCert,
			ClientKey:  m.clientKey,
			Insecure:   m.insecure,
		}
		config.TLSConfig = t
	}

	return api.NewClient(config)
}

func (m *Meta) Colorize() *colorstring.Colorize {
	return &colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: m.noColor || !terminal.IsTerminal(int(os.Stdout.Fd())),
		Reset:   true,
	}
}

// generalOptionsUsage returns the help string for the global options.
func generalOptionsUsage() string {
	helpText := `
  -address=<addr>
    The address of the Nomad server.
    Overrides the NOMAD_ADDR environment variable if set.
    Default = http://127.0.0.1:4646

  -region=<region>
    The region of the Nomad servers to forward commands to.
    Overrides the NOMAD_REGION environment variable if set.
    Defaults to the Agent's local region.

  -namespace=<namespace>
    The target namespace for queries and actions bound to a namespace.
    Overrides the NOMAD_NAMESPACE environment variable if set.
    Defaults to the "default" namespace.

  -no-color
    Disables colored command output.

  -ca-cert=<path>
    Path to a PEM encoded CA cert file to use to verify the
    Nomad server SSL certificate.  Overrides the NOMAD_CACERT
    environment variable if set.

  -ca-path=<path>
    Path to a directory of PEM encoded CA cert files to verify
    the Nomad server SSL certificate. If both -ca-cert and
    -ca-path are specified, -ca-cert is used. Overrides the
    NOMAD_CAPATH environment variable if set.

  -client-cert=<path>
    Path to a PEM encoded client certificate for TLS authentication
    to the Nomad server. Must also specify -client-key. Overrides
    the NOMAD_CLIENT_CERT environment variable if set.

  -client-key=<path>
    Path to an unencrypted PEM encoded private key matching the
    client certificate from -client-cert. Overrides the
    NOMAD_CLIENT_KEY environment variable if set.

  -tls-skip-verify
    Do not verify TLS certificate. This is highly not recommended. Verification
    will also be skipped if NOMAD_SKIP_VERIFY is set.
`
	return strings.TrimSpace(helpText)
}
