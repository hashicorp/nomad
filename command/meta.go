package command

import (
	"flag"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	colorable "github.com/mattn/go-colorable"
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

	// Whether to force colorized output
	forceColor bool

	// The region to send API requests
	region string

	// namespace to send API requests
	namespace string

	// token is used for ACLs to access privileged information
	token string

	caCert        string
	caPath        string
	clientCert    string
	clientKey     string
	tlsServerName string
	insecure      bool
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
		f.BoolVar(&m.forceColor, "force-color", false, "")
		f.StringVar(&m.caCert, "ca-cert", "", "")
		f.StringVar(&m.caPath, "ca-path", "", "")
		f.StringVar(&m.clientCert, "client-cert", "", "")
		f.StringVar(&m.clientKey, "client-key", "", "")
		f.BoolVar(&m.insecure, "insecure", false, "")
		f.StringVar(&m.tlsServerName, "tls-server-name", "", "")
		f.BoolVar(&m.insecure, "tls-skip-verify", false, "")
		f.StringVar(&m.token, "token", "", "")

	}

	f.SetOutput(&uiErrorWriter{ui: m.Ui})

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
		"-force-color":     complete.PredictNothing,
		"-ca-cert":         complete.PredictFiles("*"),
		"-ca-path":         complete.PredictDirs("*"),
		"-client-cert":     complete.PredictFiles("*"),
		"-client-key":      complete.PredictFiles("*"),
		"-insecure":        complete.PredictNothing,
		"-tls-server-name": complete.PredictNothing,
		"-tls-skip-verify": complete.PredictNothing,
		"-token":           complete.PredictAnything,
	}
}

// ApiClientFactory is the signature of a API client factory
type ApiClientFactory func() (*api.Client, error)

// Client is used to initialize and return a new API client using
// the default command line arguments and env vars.
func (m *Meta) clientConfig() *api.Config {
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
	if m.caCert != "" || m.caPath != "" || m.clientCert != "" || m.clientKey != "" || m.tlsServerName != "" || m.insecure {
		t := &api.TLSConfig{
			CACert:        m.caCert,
			CAPath:        m.caPath,
			ClientCert:    m.clientCert,
			ClientKey:     m.clientKey,
			TLSServerName: m.tlsServerName,
			Insecure:      m.insecure,
		}
		config.TLSConfig = t
	}

	if m.token != "" {
		config.SecretID = m.token
	}

	return config
}

func (m *Meta) Client() (*api.Client, error) {
	return api.NewClient(m.clientConfig())
}

func (m *Meta) allNamespaces() bool {
	return m.clientConfig().Namespace == api.AllNamespacesNamespace
}

func (m *Meta) Colorize() *colorstring.Colorize {
	_, coloredUi := m.Ui.(*cli.ColoredUi)

	return &colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !coloredUi,
		Reset:   true,
	}
}

func (m *Meta) SetupUi(args []string) {
	noColor := os.Getenv(EnvNomadCLINoColor) != ""
	forceColor := os.Getenv(EnvNomadCLIForceColor) != ""

	for _, arg := range args {
		// Check if color is set
		if arg == "-no-color" || arg == "--no-color" {
			noColor = true
		} else if arg == "-force-color" || arg == "--force-color" {
			forceColor = true
		}
	}

	m.Ui = &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      colorable.NewColorableStdout(),
		ErrorWriter: colorable.NewColorableStderr(),
	}

	// Only use colored UI if not disabled and stdout is a tty or colors are
	// forced.
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	useColor := !noColor && (isTerminal || forceColor)
	if useColor {
		m.Ui = &cli.ColoredUi{
			ErrorColor: cli.UiColorRed,
			WarnColor:  cli.UiColorYellow,
			InfoColor:  cli.UiColorGreen,
			Ui:         m.Ui,
		}
	}
}

type usageOptsFlags uint8

const (
	usageOptsDefault     usageOptsFlags = 0
	usageOptsNoNamespace                = 1 << iota
)

// generalOptionsUsage returns the help string for the global options.
func generalOptionsUsage(usageOpts usageOptsFlags) string {

	helpText := `
  -address=<addr>
    The address of the Nomad server.
    Overrides the NOMAD_ADDR environment variable if set.
    Default = http://127.0.0.1:4646

  -region=<region>
    The region of the Nomad servers to forward commands to.
    Overrides the NOMAD_REGION environment variable if set.
    Defaults to the Agent's local region.
`

	namespaceText := `
  -namespace=<namespace>
    The target namespace for queries and actions bound to a namespace.
    Overrides the NOMAD_NAMESPACE environment variable if set.
    If set to '*', job and alloc subcommands query all namespaces authorized
    to user.
    Defaults to the "default" namespace.
`

	// note: that although very few commands use color explicitly, all of them
	// return red-colored text on error so we want the color flags to always be
	// present in the help messages.
	remainingText := `
  -no-color
    Disables colored command output. Alternatively, NOMAD_CLI_NO_COLOR may be
    set. This option takes precedence over -force-color.

  -force-color
    Forces colored command output. This can be used in cases where the usual
    terminal detection fails. Alternatively, NOMAD_CLI_FORCE_COLOR may be set.
    This option has no effect if -no-color is also used.

  -ca-cert=<path>
    Path to a PEM encoded CA cert file to use to verify the
    Nomad server SSL certificate. Overrides the NOMAD_CACERT
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

  -tls-server-name=<value>
    The server name to use as the SNI host when connecting via
    TLS. Overrides the NOMAD_TLS_SERVER_NAME environment variable if set.

  -tls-skip-verify
    Do not verify TLS certificate. This is highly not recommended. Verification
    will also be skipped if NOMAD_SKIP_VERIFY is set.

  -token
    The SecretID of an ACL token to use to authenticate API requests with.
    Overrides the NOMAD_TOKEN environment variable if set.
`

	if usageOpts&usageOptsNoNamespace == 0 {
		helpText = helpText + namespaceText
	}

	helpText = helpText + remainingText
	return strings.TrimSpace(helpText)
}

// funcVar is a type of flag that accepts a function that is the string given
// by the user.
type funcVar func(s string) error

func (f funcVar) Set(s string) error { return f(s) }
func (f funcVar) String() string     { return "" }
func (f funcVar) IsBoolFlag() bool   { return false }
