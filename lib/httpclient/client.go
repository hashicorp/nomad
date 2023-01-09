package httpclient

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
)

// A Client must implement the Do method.
type Client interface {
	Do(*http.Request) (*http.Response, error)
	Get(string) (*http.Response, error)
}

// NomadHTTP is an implementation of Client that automatically configures an
// underlying http.Client with Nomad specific details.
type NomadHTTP struct {
	*http.Client

	userAgent string
}

// An Option is used to set configuration on a NomadHTTP client.
type Option func(c *NomadHTTP)

// UserAgent configures the User-Agent string to inject into every request,
// regardless of configuration of the http.Request object.
func UserAgent(ua string) Option {
	return func(f *NomadHTTP) {
		f.userAgent = ua
	}
}

// RoundTripper configures the http.RoundTripper to set as the http.Transport
// of the http.Client. Defaults to a pooled connection configured by cleanhttp.DefaultClient.
func RoundTripper(tr http.RoundTripper) Option {
	return func(f *NomadHTTP) {
		f.Transport = tr
	}
}

// Timeout configures the global timeout value on the http.Client. By default
// this is left unset - however the underlying http client is configured with
// - Transport.DialContext.Timeout
// - Transport.IdleConnTimeout
// - Transport.TLSHandshakeTimeout
// - Transport.ExpectContinueTimeout
//
// These detailed timeouts can be further configured by supplying your own http.Transport
// via RoundTripper.
func Timeout(timeout time.Duration) Option {
	return func(f *NomadHTTP) {
		f.Timeout = timeout
	}
}

// New creates an implementation of Client specific to Nomad's use case(s). It
// can be customized by specifying various Option values.
func New(options ...Option) *NomadHTTP {
	rf := &NomadHTTP{
		Client:    cleanhttp.DefaultClient(),
		userAgent: NomadUserAgent(),
	}
	for _, opt := range options {
		opt(rf)
	}
	return rf
}

func (n *NomadHTTP) setUserAgent(request *http.Request) {
	request.Header.Set("User-Agent", n.userAgent)
}

// Do executes the given request.
func (n *NomadHTTP) Do(request *http.Request) (*http.Response, error) {
	n.setUserAgent(request)
	return n.Client.Do(request)
}

// Get performs an HTTP get request to url.
func (n *NomadHTTP) Get(url string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return n.Do(request)
}
