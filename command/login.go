package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/hashicorp/cap/util"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/lib/auth/oidc"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure LoginCommand satisfies the cli.Command interface.
var _ cli.Command = &LoginCommand{}

// LoginCommand implements cli.Command.
type LoginCommand struct {
	Meta

	authMethodType string
	authMethodName string

	callbackAddr string

	template string
	json     bool
}

// Help satisfies the cli.Command Help function.
func (l *LoginCommand) Help() string {
	helpText := `
Usage: nomad login [options]

  The login command will exchange the provided third party credentials with the
  requested auth method for a newly minted Nomad ACL token.

General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `

Login Options:

  -method
    The name of the ACL auth method to login to. If the cluster administrator
    has configured a default, this flag is optional.

  -type
    Type of the auth method to login to. This is required.

  -json
    Output the ACL token in JSON format.

  -t
    Format and display the ACL token using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (l *LoginCommand) Synopsis() string {
	return "Login to Nomad using an auth method"
}

func (l *LoginCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(l.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-method":             complete.PredictAnything,
			"-type":               complete.PredictSet("OIDC"),
			"-oidc-callback-addr": complete.PredictAnything,
			"-json":               complete.PredictNothing,
			"-t":                  complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (l *LoginCommand) Name() string { return "login" }

// Run satisfies the cli.Command Run function.
func (l *LoginCommand) Run(args []string) int {

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.StringVar(&l.authMethodName, "method", "", "")
	flags.StringVar(&l.authMethodType, "type", "", "")
	flags.StringVar(&l.callbackAddr, "oidc-callback-addr", "127.0.0.1:4649", "")
	flags.BoolVar(&l.json, "json", false, "")
	flags.StringVar(&l.template, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) != 0 {
		l.Ui.Error("This command takes no arguments")
		l.Ui.Error(commandErrorText(l))
		return 1
	}

	// Auth method types are particular with their naming, so ensure we forgive
	// any case mistakes here from the user.
	sanitizedMethodType := strings.ToUpper(l.authMethodType)

	switch sanitizedMethodType {
	case "":
		l.Ui.Error(fmt.Sprintf("Please supply an authentication type"))
		return 1
	case api.ACLAuthMethodTypeOIDC:
	default:
		l.Ui.Error(fmt.Sprintf("Unsupported authentication type %q", sanitizedMethodType))
		return 1
	}

	client, err := l.Meta.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if l.authMethodName == "" {

		authMethodList, _, err := client.ACLAuthMethods().List(nil)
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error listing ACL auth methods: %s", err))
			return 1
		}

		for _, authMethod := range authMethodList {
			if authMethod.Default {
				l.authMethodName = authMethod.Name
			}
		}

		if l.authMethodName == "" {
			l.Ui.Error("Must specify an auth method name, no default found")
			return 1
		}
	}

	// Validate other flags to ensure they are correctly set.
	// Switch on the auth type and call the sub-function; only OIDC now.
	// Format the ACL token.

	var authFn func(context.Context, *api.Client) (*api.ACLToken, error)

	switch sanitizedMethodType {
	case api.ACLAuthMethodTypeOIDC:
		authFn = l.loginOIDC
	default:
		l.Ui.Error(fmt.Sprintf("Unsupported authentication type %q", sanitizedMethodType))
		return 1
	}

	ctx, cancel := contextWithInterrupt()
	defer cancel()

	token, err := authFn(ctx, client)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error performing login: %v", err))
		return 1
	}

	if l.json || l.template != "" {
		out, err := Format(l.json, l.template, token)
		if err != nil {
			l.Ui.Error(err.Error())
			return 1
		}
		l.Ui.Output(out)
		return 0
	}

	l.Ui.Output(fmt.Sprintf("Successfully logged in via %s and %s\n", sanitizedMethodType, l.authMethodName))
	outputACLToken(l.Ui, token)
	return 0
}

func (l *LoginCommand) loginOIDC(ctx context.Context, client *api.Client) (*api.ACLToken, error) {

	callbackServer, err := oidc.NewCallbackServer(l.callbackAddr)
	if err != nil {
		return nil, err
	}

	getAuthArgs := api.ACLOIDCAuthURLRequest{
		AuthMethodName: l.authMethodName,
		RedirectURI:    callbackServer.RedirectURI(),
		ClientNonce:    callbackServer.Nonce(),
	}

	getAuthURLResp, _, err := client.ACLOIDC().GetAuthUrl(&getAuthArgs, nil)
	if err != nil {
		return nil, err
	}

	// Open the auth URL in the user browser or ask them to visit it.
	// We purposely use fmt here and NOT c.ui because the ui will truncate
	// our URL (a known bug).
	if err := util.OpenURL(getAuthURLResp.AuthURL); err != nil {
		l.Ui.Error(fmt.Sprintf("Error opening OIDC provider URL: %v\n", err))
		l.Ui.Output(fmt.Sprintf(strings.TrimSpace(outVisitURL)+"\n\n", getAuthURLResp.AuthURL))
	}

	// Wait
	var req *api.ACLOIDCCompleteAuthRequest
	select {
	case <-ctx.Done():
		_ = callbackServer.Close()
		return nil, ctx.Err()
	case err := <-callbackServer.ErrorCh():
		return nil, err
	case req = <-callbackServer.SuccessCh():
	}

	cbArgs := api.ACLOIDCCompleteAuthRequest{
		AuthMethodName: l.authMethodName,
		RedirectURI:    callbackServer.RedirectURI(),
		ClientNonce:    callbackServer.Nonce(),
		Code:           req.Code,
		State:          req.State,
	}

	token, _, err := client.ACLOIDC().CompleteAuth(&cbArgs, nil)
	return token, err
}

const (
	outVisitURL = `
Complete the authentication by visiting your authentication provider using the
URL below:

%s
`
)

// contextWithInterrupt
func contextWithInterrupt() (context.Context, func()) {

	// Create the cancellable context that we'll use when we receive an
	// interrupt.
	ctx, cancel := context.WithCancel(context.Background())

	// Create the signal channel and cancel the context when we get a signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Start a routine which waits for the signals.
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	// Return the context and a closer that cancels the context and also
	// stops any signals from coming to our channel.
	return ctx, func() {
		signal.Stop(ch)
		cancel()
	}
}
