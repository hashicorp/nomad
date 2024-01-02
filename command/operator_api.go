// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

// Stdin represents the system's standard input, but it's declared as a
// variable here to allow tests to override it with a regular file.
var Stdin = os.Stdin

type OperatorAPICommand struct {
	Meta

	verboseFlag bool
	method      string
	body        io.Reader
}

func (*OperatorAPICommand) Help() string {
	helpText := `
Usage: nomad operator api [options] <path>

  api is a utility command for accessing Nomad's HTTP API and is inspired by
  the popular curl command line tool. Nomad's operator api command populates
  Nomad's standard environment variables into their appropriate HTTP headers.
  If the 'path' does not begin with "http" then $NOMAD_ADDR will be used.

  The 'path' can be in one of the following forms:

    /v1/allocations                       <- API Paths must start with a /
    localhost:4646/v1/allocations         <- Scheme will be inferred
    https://localhost:4646/v1/allocations <- Scheme will be https://

  Note that this command does not always match the popular curl program's
  behavior. Instead Nomad's operator api command is optimized for common Nomad
  HTTP API operations.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Operator API Specific Options:

  -dryrun
    Output equivalent curl command to stdout and exit.
    HTTP Basic Auth will never be output. If the $NOMAD_HTTP_AUTH environment
    variable is set, it will be referenced in the appropriate curl flag in the
    output.
    ACL tokens set via the $NOMAD_TOKEN environment variable will only be
    referenced by environment variable as with HTTP Basic Auth above. However
    if the -token flag is explicitly used, the token will also be included in
    the output.

  -filter <query>
    Specifies an expression used to filter query results.

  -H <Header>
    Adds an additional HTTP header to the request. May be specified more than
    once. These headers take precedence over automatically set ones such as
    X-Nomad-Token.

  -verbose
    Output extra information to stderr similar to curl's --verbose flag.

  -X <HTTP Method>
    HTTP method of request. If there is data piped to stdin, then the method
    defaults to POST. Otherwise the method defaults to GET.
`

	return strings.TrimSpace(helpText)
}

func (*OperatorAPICommand) Synopsis() string {
	return "Query Nomad's HTTP API"
}

func (c *OperatorAPICommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-dryrun": complete.PredictNothing,
		})
}

func (c *OperatorAPICommand) AutocompleteArgs() complete.Predictor {
	//TODO(schmichael) wouldn't it be cool to build path autocompletion off
	//                 of our http mux?
	return complete.PredictNothing
}

func (*OperatorAPICommand) Name() string { return "operator api" }

func (c *OperatorAPICommand) Run(args []string) int {
	var dryrun bool
	var filter string
	headerFlags := newHeaderFlags()

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&dryrun, "dryrun", false, "")
	flags.StringVar(&filter, "filter", "", "")
	flags.BoolVar(&c.verboseFlag, "verbose", false, "")
	flags.StringVar(&c.method, "X", "", "")
	flags.Var(headerFlags, "H", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}
	args = flags.Args()

	if len(args) < 1 {
		c.Ui.Error("A path or URL is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if n := len(args); n > 1 {
		c.Ui.Error(fmt.Sprintf("operator api accepts exactly 1 argument, but %d arguments were found", n))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// By default verbose func is a noop
	verbose := func(string, ...interface{}) {}
	if c.verboseFlag {
		verbose = func(format string, a ...interface{}) {
			// Use Warn instead of Info because Info goes to stdout
			c.Ui.Warn(fmt.Sprintf(format, a...))
		}
	}

	// Opportunistically read from stdin and POST unless method has been
	// explicitly set.
	stat, _ := Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		verbose("* Reading request body from stdin.")

		// Load stdin into a *bytes.Reader so that http.NewRequest can set the
		// correct Content-Length value.
		b, err := io.ReadAll(Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading stdin: %v", err))
			return 1
		}
		c.body = bytes.NewReader(b)
		if c.method == "" {
			c.method = "POST"
		}
	} else if c.method == "" {
		c.method = "GET"
	}

	config := c.clientConfig()

	// NewClient mutates or validates Config.Address, so call it to match
	// the behavior of other commands.
	_, err := api.NewClient(config)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	path, err := pathToURL(config, args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error turning path into URL: %v", err))
		return 1
	}

	// Set Filter query param
	if filter != "" {
		q := path.Query()
		q.Set("filter", filter)
		path.RawQuery = q.Encode()
	}

	if dryrun {
		out, err := c.apiToCurl(config, headerFlags.headers, path)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating curl command: %v", err))
			return 1
		}
		c.Ui.Output(out)
		return 0
	}

	// Re-implement a big chunk of api/api.go since we don't export it.
	client := cleanhttp.DefaultClient()
	transport := client.Transport.(*http.Transport)
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if err := api.ConfigureTLS(client, config.TLSConfig); err != nil {
		c.Ui.Error(fmt.Sprintf("Error configuring TLS: %v", err))
		return 1
	}

	setQueryParams(config, path)

	verbose("> %s %s", c.method, path)

	req, err := http.NewRequest(c.method, path.String(), c.body)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error making request: %v", err))
		return 1
	}

	// Set headers from command line
	req.Header = headerFlags.headers

	// Add token header if it doesn't already exist and is set
	if req.Header.Get("X-Nomad-Token") == "" && config.SecretID != "" {
		req.Header.Set("X-Nomad-Token", config.SecretID)
	}

	// Configure HTTP basic authentication if set
	if path.User != nil {
		username := path.User.Username()
		password, _ := path.User.Password()
		req.SetBasicAuth(username, password)
	} else if config.HttpAuth != nil {
		req.SetBasicAuth(config.HttpAuth.Username, config.HttpAuth.Password)
	}

	for k, vals := range req.Header {
		for _, v := range vals {
			verbose("> %s: %s", k, v)
		}
	}

	verbose("* Sending request and receiving response...")

	// Do the request!
	resp, err := client.Do(req)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error performing request: %v", err))
		return 1
	}
	defer resp.Body.Close()

	verbose("< %s %s", resp.Proto, resp.Status)
	for k, vals := range resp.Header {
		for _, v := range vals {
			verbose("< %s: %s", k, v)
		}
	}

	n, err := io.Copy(os.Stdout, resp.Body)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading response after %d bytes: %v", n, err))
		return 1
	}

	if len(resp.Trailer) > 0 {
		verbose("* Trailer Headers")
		for k, vals := range resp.Trailer {
			for _, v := range vals {
				verbose("< %s: %s", k, v)
			}
		}
	}

	return 0
}

// setQueryParams converts API configuration to query parameters. Updates path
// parameter in place.
func setQueryParams(config *api.Config, path *url.URL) {
	queryParams := path.Query()

	// Prefer region explicitly set in path, otherwise fallback to config
	// if one is set.
	if queryParams.Get("region") == "" && config.Region != "" {
		queryParams["region"] = []string{config.Region}
	}

	// Prefer namespace explicitly set in path, otherwise fallback to
	// config if one is set.
	if queryParams.Get("namespace") == "" && config.Namespace != "" {
		queryParams["namespace"] = []string{config.Namespace}
	}

	// Re-encode query parameters
	path.RawQuery = queryParams.Encode()
}

// apiToCurl converts a Nomad HTTP API config and path to its corresponding
// curl command or returns an error.
func (c *OperatorAPICommand) apiToCurl(config *api.Config, headers http.Header, path *url.URL) (string, error) {
	parts := []string{"curl"}

	if c.verboseFlag {
		parts = append(parts, "--verbose")
	}

	if c.method != "" {
		parts = append(parts, "-X "+c.method)
	}

	if c.body != nil {
		parts = append(parts, "--data-binary @-")
	}

	if config.TLSConfig != nil {
		parts = tlsToCurl(parts, config.TLSConfig)

		// If a TLS server name is set we must alter the URL and use
		// curl's --connect-to flag.
		if v := config.TLSConfig.TLSServerName; v != "" {
			pathHost, port, err := net.SplitHostPort(path.Host)
			if err != nil {
				return "", fmt.Errorf("error determining port: %v", err)
			}

			// curl uses the url for SNI so override it with the
			// configured server name
			path.Host = net.JoinHostPort(v, port)

			// curl uses --connect-to to allow specifying a
			// different connection address for the hostname in the
			// path. The format is:
			//   logical-host:logical-port:actual-host:actual-port
			// Ports will always match since only the hostname is
			// overridden for SNI.
			parts = append(parts, fmt.Sprintf(`--connect-to "%s:%s:%s:%s"`,
				v, port, pathHost, port))
		}
	}

	// Add headers
	for k, vals := range headers {
		for _, v := range vals {
			parts = append(parts, fmt.Sprintf(`-H '%s: %s'`, k, v))
		}
	}

	// Only write NOMAD_TOKEN to stdout if it was specified via -token.
	// Otherwise output a static string that references the ACL token
	// environment variable.
	if headers.Get("X-Nomad-Token") == "" {
		if c.Meta.token != "" {
			parts = append(parts, fmt.Sprintf(`-H 'X-Nomad-Token: %s'`, c.Meta.token))
		} else if v := os.Getenv("NOMAD_TOKEN"); v != "" {
			parts = append(parts, `-H "X-Nomad-Token: ${NOMAD_TOKEN}"`)
		}
	}

	// Never write http auth to stdout. Instead output a static string that
	// references the HTTP auth environment variable.
	if auth := os.Getenv("NOMAD_HTTP_AUTH"); auth != "" {
		parts = append(parts, `-u "$NOMAD_HTTP_AUTH"`)
	}

	setQueryParams(config, path)

	parts = append(parts, path.String())

	return strings.Join(parts, " \\\n  "), nil
}

// tlsToCurl converts TLS configuration to their corresponding curl flags.
func tlsToCurl(parts []string, tlsConfig *api.TLSConfig) []string {
	if v := tlsConfig.CACert; v != "" {
		parts = append(parts, fmt.Sprintf(`--cacert "%s"`, v))
	}

	if v := tlsConfig.CAPath; v != "" {
		parts = append(parts, fmt.Sprintf(`--capath "%s"`, v))
	}

	if v := tlsConfig.ClientCert; v != "" {
		parts = append(parts, fmt.Sprintf(`--cert "%s"`, v))
	}

	if v := tlsConfig.ClientKey; v != "" {
		parts = append(parts, fmt.Sprintf(`--key "%s"`, v))
	}

	// TLSServerName has already been configured as it may change the path.

	if tlsConfig.Insecure {
		parts = append(parts, `--insecure`)
	}

	return parts
}

// pathToURL converts a curl path argument to URL. Paths without a host are
// prefixed with $NOMAD_ADDR or http://127.0.0.1:4646.
//
// Callers should pass a config generated by Meta.clientConfig which ensures
// all default values are set correctly. Failure to do so will likely result in
// a nil-pointer.
func pathToURL(config *api.Config, path string) (*url.URL, error) {

	// If the scheme is missing from the path, it likely means the path is just
	// the HTTP handler path. Attempt to infer this.
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		scheme := "http"

		// If the user has set any TLS configuration value, this is a good sign
		// Nomad is running with TLS enabled. Otherwise, use the address within
		// the config to identify a scheme.
		if config.TLSConfig.CACert != "" ||
			config.TLSConfig.CAPath != "" ||
			config.TLSConfig.ClientCert != "" ||
			config.TLSConfig.TLSServerName != "" ||
			config.TLSConfig.Insecure {

			// TLS configured, but scheme not set. Assume https.
			scheme = "https"
		} else if config.Address != "" {

			confURL, err := url.Parse(config.Address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse configured address: %v", err)
			}

			// Ensure we only overwrite the set scheme value if the parsing
			// identified a valid scheme.
			if confURL.Scheme == "http" || confURL.Scheme == "https" {
				scheme = confURL.Scheme
			}
		}

		path = fmt.Sprintf("%s://%s", scheme, path)
	}

	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	// If URL.Host is empty, use defaults from client config.
	if u.Host == "" {
		confURL, err := url.Parse(config.Address)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse configured address: %v", err)
		}
		u.Host = confURL.Host
	}

	return u, nil
}

// headerFlags is a flag.Value implementation for collecting multiple -H flags.
type headerFlags struct {
	headers http.Header
}

func newHeaderFlags() *headerFlags {
	return &headerFlags{
		headers: make(http.Header),
	}
}

func (*headerFlags) String() string { return "" }

func (h *headerFlags) Set(v string) error {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Headers must be in the form 'Key: Value' but found: %q", v)
	}

	h.headers.Add(parts[0], strings.TrimSpace(parts[1]))
	return nil
}
