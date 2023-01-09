package httpclient

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
)

// A Client must implement the Do method.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

// NomadHTTP is an implementation of Client that automatically configures an
// underlying http.Client with Nomad specific details.
type NomadHTTP struct {
	userAgent string
	client    *http.Client
}

// An Option is used to set configuration on a NomadHTTP client.
type Option func(c *NomadHTTP)

// WithUserAgent configures the User-Agent string to inject into every request,
// regardless of configuration of the http.Request object.
func WithUserAgent(ua string) Option {
	return func(f *NomadHTTP) {
		f.userAgent = ua
	}
}

// WithRoundTripper configures the http.RoundTripper to set as the http.Transport
// of the http.Client. Defaults to a pooled connection configured by cleanhttp.DefaultClient.
func WithRoundTripper(tr http.RoundTripper) Option {
	return func(f *NomadHTTP) {
		f.client.Transport = tr
	}
}

// WithTimeout configures the global timeout value on the http.Client. By default
// this is left unset - however the underlying http client is configured with
// - Transport.DialContext.Timeout
// - Transport.IdleConnTimeout
// - Transport.TLSHandshakeTimeout
// - Transport.ExpectContinueTimeout
//
// These detailed timeouts can be further configured by supplying your own http.Transport
// via WithRoundTripper.
func WithTimeout(timeout time.Duration) Option {
	return func(f *NomadHTTP) {
		f.client.Timeout = timeout
	}
}

// New creates an implementation of Client specific to Nomad's use case(s). It
// can be customized by specifying various Option values.
func New(options ...Option) *NomadHTTP {
	rf := &NomadHTTP{
		userAgent: defaultUserAgent(),
		client:    cleanhttp.DefaultClient(),
	}
	for _, opt := range options {
		opt(rf)
	}
	return rf
}

// Do executes the given request.
func (n *NomadHTTP) Do(request *http.Request) (*http.Response, error) {
	request.Header.Set("User-Agent", n.userAgent)
	return n.client.Do(request)
}
