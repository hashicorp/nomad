package command

import (
	"flag"
	"os"

	"github.com/hashicorp/nomad/api"
)

const (
	// Environment variable used to control the HTTP address
	// we connect to while using various commands. This may
	// be overridden using the -http-addr flag.
	HttpEnvVar = "NOMAD_HTTP_ADDR"

	// DefaultHTTPAddr is the default address used for the
	// HTTP address flag.
	DefaultHttpAddr = "http://127.0.0.1:4646"
)

// httpAddrFlag is used to add the -http-addr flag to a flag
// set. Allows setting the value from an environment variable.
func httpAddrFlag(f *flag.FlagSet) *string {
	defaultAddr := os.Getenv(HttpEnvVar)
	if defaultAddr == "" {
		defaultAddr = DefaultHttpAddr
	}
	return f.String("http-addr", defaultAddr,
		"HTTP address of the Nomad agent")
}

// httpClient is used to get a new Nomad client using the
// given address.
func httpClient(addr string) (*api.Client, error) {
	conf := api.DefaultConfig()
	conf.URL = addr
	return api.NewClient(conf)
}
