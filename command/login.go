// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/hashicorp/cap/util"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/lib/auth/oidc"
)

// Ensure LoginCommand satisfies the cli.Command interface.
var _ cli.Command = &LoginCommand{}

// LoginCommand implements cli.Command.
type LoginCommand struct {
	Meta

	authMethodType string // deprecated in 1.5.2, left for backwards compat
	authMethodName string
	callbackAddr   string
	loginToken     string

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

  -oidc-callback-addr
    The address to use for the local OIDC callback server. This should be given
    in the form of <IP>:<PORT> and defaults to "localhost:4649".

  -login-token
    Login token used for authentication that will be exchanged for a Nomad ACL
    Token. It is only required if using auth method type other than OIDC. 

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
			"-oidc-callback-addr": complete.PredictAnything,
			"-login-token":        complete.PredictAnything,
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
	flags.StringVar(&l.loginToken, "login-token", "", "")
	flags.StringVar(&l.callbackAddr, "oidc-callback-addr", "localhost:4649", "")
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

	client, err := l.Meta.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	var (
		defaultMethod *api.ACLAuthMethodListStub
		methodType    string
	)

	if l.authMethodType != "" {
		l.Ui.Warn("warning: '-type' flag has been deprecated for nomad login command and will be ignored.")
	}

	authMethodList, _, err := client.ACLAuthMethods().List(nil)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error listing ACL auth methods: %s", err))
		return 1
	}

	for _, authMethod := range authMethodList {
		if authMethod.Default {
			defaultMethod = authMethod
		}
	}

	// If there is a default method available, and the caller did not pass method
	// name, fill it in. In case there is no default method, error and quit.
	if l.authMethodName == "" {
		if defaultMethod != nil {
			l.authMethodName = defaultMethod.Name
			methodType = defaultMethod.Type
		} else {
			l.Ui.Error("Must specify an auth method name, no default found")
			return 1
		}
	} else {
		// Find the method by name in the state store and get its type
		for _, method := range authMethodList {
			if method.Name == l.authMethodName {
				methodType = method.Type
			}
		}

		if methodType == "" {
			l.Ui.Error(fmt.Sprintf(
				"Error: method %s not found in the state store. ",
				l.authMethodName,
			))
			return 1
		}
	}

	// Make sure we got the login token if we're not using OIDC
	if methodType != api.ACLAuthMethodTypeOIDC && l.loginToken == "" {
		l.Ui.Error("You need to provide a login token.")
		return 1
	}

	// Each login type should implement a function which matches this signature
	// for the specific login implementation. This allows the command to have
	// reusable and generic handling of errors and outputs.
	var authFn func(context.Context, *api.Client) (*api.ACLToken, error)

	switch methodType {
	case api.ACLAuthMethodTypeOIDC:
		authFn = l.loginOIDC
	case api.ACLAuthMethodTypeJWT:
		authFn = l.loginJWT
	default:
		l.Ui.Error(fmt.Sprintf("Unsupported authentication type %q", methodType))
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

	l.Ui.Output(fmt.Sprintf("Successfully logged in via %s and %s\n", methodType, l.authMethodName))
	outputACLToken(l.Ui, token)
	return 0
}

func (l *LoginCommand) loginOIDC(ctx context.Context, client *api.Client) (*api.ACLToken, error) {

	callbackServer, err := oidc.NewCallbackServer(l.callbackAddr)
	if err != nil {
		return nil, err
	}
	defer callbackServer.Close()

	getAuthArgs := api.ACLOIDCAuthURLRequest{
		AuthMethodName: l.authMethodName,
		RedirectURI:    callbackServer.RedirectURI(),
		ClientNonce:    callbackServer.Nonce(),
	}

	getAuthURLResp, _, err := client.ACLAuth().GetAuthURL(&getAuthArgs, nil)
	if err != nil {
		return nil, err
	}

	// Open the auth URL in the user browser or ask them to visit it.
	// We purposely use fmt here and NOT c.ui because the ui will truncate
	// our URL (a known bug).
	if err := util.OpenURL(getAuthURLResp.AuthURL); err != nil {
		l.Ui.Error(fmt.Sprintf("Error opening OIDC provider URL: %v\n", err))
		l.Ui.Output(fmt.Sprintf(strings.TrimSpace(oidcErrorVisitURLMsg)+"\n\n", getAuthURLResp.AuthURL))
	}

	// Wait. The login process can end to one of the following reasons:
	// - the user interrupts the login process via CTRL-C
	// - the login process returns an error via the callback server
	// - the login process is successful as returned by the callback server
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

	token, _, err := client.ACLAuth().CompleteAuth(&cbArgs, nil)
	return token, err
}

func (l *LoginCommand) loginJWT(ctx context.Context, client *api.Client) (*api.ACLToken, error) {
	authArgs := api.ACLLoginRequest{
		AuthMethodName: l.authMethodName,
		LoginToken:     l.loginToken,
	}
	token, _, err := client.ACLAuth().Login(&authArgs, nil)
	return token, err
}

const (
	// oidcErrorVisitURLMsg is a message to show users when opening the OIDC
	// provider URL automatically fails. This type of message is otherwise not
	// needed, as it just clutters the console without providing value.
	oidcErrorVisitURLMsg = `
Automatic opening of the OIDC provider for login has failed. To complete the
authentication, please visit your provider using the URL below:

%s
`
)

// contextWithInterrupt returns a context and cancel function that adheres to
// expected behaviour and also includes cancellation when the user interrupts
// the login process via CTRL-C.
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
