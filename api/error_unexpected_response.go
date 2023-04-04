package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UnexpectedResponseError tracks the components for API errors encountered when
// requireOK and requireStatusIn's conditions are not met.
type UnexpectedResponseError struct {
	expected   []int
	statusCode int
	statusText string
	body       string
	err        error
	additional error
}

func (e UnexpectedResponseError) HasExpectedStatuses() bool { return len(e.expected) > 0 }
func (e UnexpectedResponseError) ExpectedStatuses() []int   { return e.expected }
func (e UnexpectedResponseError) HasStatusCode() bool       { return e.statusCode != 0 }
func (e UnexpectedResponseError) StatusCode() int           { return e.statusCode }
func (e UnexpectedResponseError) HasStatusText() bool       { return e.statusText != "" }
func (e UnexpectedResponseError) StatusText() string        { return e.statusText }
func (e UnexpectedResponseError) HasBody() bool             { return e.body != "" }
func (e UnexpectedResponseError) Body() string              { return e.body }
func (e UnexpectedResponseError) HasError() bool            { return e.err != nil }
func (e UnexpectedResponseError) Unwrap() error             { return e.err }
func (e UnexpectedResponseError) HasAdditional() bool       { return e.additional != nil }
func (e UnexpectedResponseError) Additional() error         { return e.additional }
func NewUnexpectedResponseError(src UnexpectedResponseErrorSource, opts ...UnexpectedResponseErrorOption) UnexpectedResponseError {
	new := src()
	for _, opt := range opts {
		opt(new)
	}
	if new.statusText == "" {
		// the stdlib's http.StatusText function is a good place to start
		new.statusFromCode(http.StatusText)
	}

	return *new
}

// Use textual representation of the given integer code. Called when status text
// is not set using the WithStatusText option.
func (e UnexpectedResponseError) statusFromCode(f func(int) string) {
	e.statusText = f(e.statusCode)
	if !e.HasStatusText() {
		e.statusText = "unknown status code"
	}
}

func (e UnexpectedResponseError) Error() string {
	var eTxt strings.Builder
	eTxt.WriteString("Unexpected response code")
	if e.HasBody() || e.HasStatusCode() {
		eTxt.WriteString(": ")
	}
	if e.HasStatusCode() {
		eTxt.WriteString(fmt.Sprint(e.statusCode))
		if e.HasBody() {
			eTxt.WriteRune(' ')
		}
	}
	if e.HasBody() {
		eTxt.WriteString(fmt.Sprintf("(%s)", e.body))
	}

	if e.HasAdditional() {
		eTxt.WriteString(fmt.Sprintf(". Additionally, an error occurred while constructing this error (%s); the body might be truncated or missing.", e.additional.Error()))
	}

	return eTxt.String()
}

// UnexpectedResponseErrorOptions are functions passed to NewUnexpectedResponseError
// to customize the created error.
type UnexpectedResponseErrorOption func(*UnexpectedResponseError)

// WithError allows the addition of a Go error that may have been encountered
// while processing the response. For example, if there is an error constructing
// the gzip reader to process a gzip-encoded response body.
func WithError(e error) UnexpectedResponseErrorOption {
	return func(u *UnexpectedResponseError) { u.err = e }
}

// WithBody overwrites the Body value with the provided custom value
func WithBody(b string) UnexpectedResponseErrorOption {
	return func(u *UnexpectedResponseError) { u.body = b }
}

// WithStatusText overwrites the StatusText value the provided custom value
func WithStatusText(st string) UnexpectedResponseErrorOption {
	return func(u *UnexpectedResponseError) { u.statusText = st }
}

// WithExpectedStatuses provides a list of statuses that the receiving function
// expected to receive. This can be used by API callers to provide more feedback
// to end-users.
func WithExpectedStatuses(s []int) UnexpectedResponseErrorOption {
	return func(u *UnexpectedResponseError) {
		u.expected = make([]int, len(s))
		copy(u.expected, s)
	}
}

// UnexpectedResponseErrorSource provides the basis for a NewUnexpectedResponseError.
type UnexpectedResponseErrorSource func() *UnexpectedResponseError

// FromHTTPResponse read an open HTTP response, drains and closes its body as
// the data for the UnexpectedResponseError.
func FromHTTPResponse(resp *http.Response) UnexpectedResponseErrorSource {
	return func() *UnexpectedResponseError {
		u := new(UnexpectedResponseError)

		if resp != nil {
			// collect and close the body
			var buf bytes.Buffer
			if _, e := io.Copy(&buf, resp.Body); e != nil {
				u.additional = e
			}

			// Body has been tested as safe to close more than once
			_ = resp.Body.Close()
			body := strings.TrimSpace(buf.String())

			// make and return the error
			u.statusCode = resp.StatusCode
			u.statusText = strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode)))
			u.body = body
		}
		return u
	}
}

// FromStatusCode is the "thinnest" source for an UnexpectedResultError. It
// will attempt to resolve the status code to status text using a resolving
// function provided inside of the NewUnexpectedResponseError implementation.
func FromStatusCode(sc int) UnexpectedResponseErrorSource {
	return func() *UnexpectedResponseError { return &UnexpectedResponseError{statusCode: sc} }
}

// doRequestWrapper is a function that wraps the client's doRequest method
// and can be used to provide error and response handling
type doRequestWrapper = func(time.Duration, *http.Response, error) (time.Duration, *http.Response, error)

// requireOK is used to wrap doRequest and check for a 200
func requireOK(d time.Duration, resp *http.Response, e error) (time.Duration, *http.Response, error) {
	f := requireStatusIn(http.StatusOK)
	return f(d, resp, e)
}

// requireStatusIn is a doRequestWrapper generator that takes expected HTTP
// response codes and validates that the received response code is among them
func requireStatusIn(statuses ...int) doRequestWrapper {
	return func(d time.Duration, resp *http.Response, e error) (time.Duration, *http.Response, error) {
		if e != nil {
			if resp != nil {
				_ = resp.Body.Close()
			}
			return d, nil, e
		}

		for _, status := range statuses {
			if resp.StatusCode == status {
				return d, resp, nil
			}
		}

		return d, nil, NewUnexpectedResponseError(FromHTTPResponse(resp), WithExpectedStatuses(statuses))
	}
}
