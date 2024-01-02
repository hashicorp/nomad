// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package noxssrw (No XSS ResponseWriter) behaves like the Go standard
// library's ResponseWriter by detecting the Content-Type of a response if it
// has not been explicitly set. However, unlike the standard library's
// implementation, this implementation will never return the "text/html"
// Content-Type and instead return "text/plain".
package noxssrw

import (
	"net/http"
	"strings"
)

var (
	// DefaultUnsafeTypes are Content-Types that browsers will render as hypertext.
	// Any Content-Types that allow Javascript or remote resource fetching must be
	// converted to a Content-Type that prevents evaluation.
	//
	// Types are prefix matched to avoid comparing against specific
	// character sets (eg "text/html; charset=utf-8") which may be user
	// controlled.
	DefaultUnsafeTypes = map[string]string{
		"text/html":      "text/plain",
		"text/xhtml":     "text/plain",
		"text/xhtml+xml": "text/plain",
	}

	// DefaultHeaders contain CORS headers meant to prevent the execution
	// of Javascript in compliant browsers.
	DefaultHeaders = map[string]string{
		"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; sandbox",
		"X-Content-Type-Options":  "nosniff",
		"X-XSS-Protection":        "1; mode=block",
	}
)

// NoXSSResponseWriter implements http.ResponseWriter but prevents renderable
// Content-Types from being automatically detected. Create with
// NewResponseWriter.
type NoXSSResponseWriter struct {
	// TypeMap maps types unsafe for untrusted content to their safe
	// version; may be replaced but not mutated.
	TypeMap map[string]string

	// DefaultHeaders to set on first write if they are not already
	// explicitly set.
	DefaultHeaders map[string]string

	// buffer up to 512 bytes before detecting Content-Type and writing
	// response.
	buf []byte

	// subsequentWrite is true after the first Write is called
	subsequentWrite bool

	// flushed is true if Content-Type has been set and Writes may be
	// passed through.
	flushed bool

	// original ResponseWriter being wrapped
	orig http.ResponseWriter
}

// Header returns the header map that will be sent by
// WriteHeader. The Header map also is the mechanism with which
// Handlers can set HTTP trailers.
//
// Changing the header map after a call to WriteHeader (or
// Write) has no effect unless the modified headers are
// trailers.
//
// There are two ways to set Trailers. The preferred way is to
// predeclare in the headers which trailers you will later
// send by setting the "Trailer" header to the names of the
// trailer keys which will come later. In this case, those
// keys of the Header map are treated as if they were
// trailers. See the example. The second way, for trailer
// keys not known to the Handler until after the first Write,
// is to prefix the Header map keys with the TrailerPrefix
// constant value. See TrailerPrefix.
//
// To suppress automatic response headers (such as "Date"), set
// their value to nil.
func (w *NoXSSResponseWriter) Header() http.Header {
	return w.orig.Header()
}

// Write writes the data to the connection as part of an HTTP reply.
//
// If WriteHeader has not yet been called, Write calls
// WriteHeader(http.StatusOK) before writing the data. If the Header
// does not contain a Content-Type line, Write adds a Content-Type set
// to the result of passing the initial 512 bytes of written data to
// DetectContentType. Additionally, if the total size of all written
// data is under a few KB and there are no Flush calls, the
// Content-Length header is added automatically.
//
// Depending on the HTTP protocol version and the client, calling
// Write or WriteHeader may prevent future reads on the
// Request.Body. For HTTP/1.x requests, handlers should read any
// needed request body data before writing the response. Once the
// headers have been flushed (due to either an explicit Flusher.Flush
// call or writing enough data to trigger a flush), the request body
// may be unavailable. For HTTP/2 requests, the Go HTTP server permits
// handlers to continue to read the request body while concurrently
// writing the response. However, such behavior may not be supported
// by all HTTP/2 clients. Handlers should read before writing if
// possible to maximize compatibility.
func (w *NoXSSResponseWriter) Write(p []byte) (int, error) {
	headers := w.Header()
	// If first write, set any unset default headers. Do this on first write
	// to allow overriding the default set of headers.
	if !w.subsequentWrite {
		for k, v := range w.DefaultHeaders {
			if headers.Get(k) == "" {
				headers.Set(k, v)
			}
		}
		w.subsequentWrite = true
	}

	// If already flushed, write-through and short-circuit
	if w.flushed {
		return w.orig.Write(p)
	}

	// < 512 bytes available, buffer and wait for closing or a subsequent
	// request
	if len(w.buf)+len(p) < 512 {
		w.buf = append(w.buf, p...)
		return len(p), nil
	}

	// >= 512 bytes available, set the Content-Type and flush.
	all := append(w.buf, p...) //nolint:gocritic
	contentType := http.DetectContentType(all)

	// Prefix match to exclude the character set which may be user
	// controlled.
	for prefix, safe := range w.TypeMap {
		if strings.HasPrefix(contentType, prefix) {
			contentType = safe
			break
		}
	}

	// Set the Content-Type iff it was not already explicitly set
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", contentType)
	}

	// Write the buffer
	n, err := w.orig.Write(w.buf)
	if err != nil {
		// Throw away part of buffer written successfully and
		// inform caller p was not written at all
		w.buf = w.buf[:n]
		return 0, err
	}

	// Headers and buffer were written, this writer has been
	// flushed and can be a passthrough
	w.flushed = true

	// Write p
	return w.orig.Write(p)
}

// Close and flush the writer. Necessary for responses that never reached 512
// bytes.
func (w *NoXSSResponseWriter) Close() (int, error) {
	// If the buffer was already flushed this is a noop
	if w.flushed {
		return 0, nil
	}

	// Prefix match to exclude the character set which may be user
	// controlled.
	contentType := http.DetectContentType(w.buf)
	for prefix, safe := range w.TypeMap {
		if strings.HasPrefix(contentType, prefix) {
			contentType = safe
			break
		}
	}

	// Set the Content-Type iff it was not already explicitly set
	if headers := w.Header(); headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", contentType)
	}

	// Write the buffer
	return w.orig.Write(w.buf)
}

// WriteHeader sends an HTTP response header with the provided
// status code.
//
// If WriteHeader is not called explicitly, the first call to Write
// will trigger an implicit WriteHeader(http.StatusOK).
// Thus explicit calls to WriteHeader are mainly used to
// send error codes.
//
// The provided code must be a valid HTTP 1xx-5xx status code.
// Only one header may be written. Go does not currently
// support sending user-defined 1xx informational headers,
// with the exception of 100-continue response header that the
// Server sends automatically when the Request.Body is read.
func (w *NoXSSResponseWriter) WriteHeader(statusCode int) {
	w.orig.WriteHeader(statusCode)
}

// NewResponseWriter creates a new ResponseWriter and Close func which will
// prevent Go's http.ResponseWriter default behavior of detecting the
// Content-Type.
//
// The Close func must be called to ensure that responses < 512 bytes are
// flushed as up to 512 bytes are buffered without flushing.
func NewResponseWriter(orig http.ResponseWriter) (http.ResponseWriter, func() (int, error)) {
	w := &NoXSSResponseWriter{
		TypeMap:        DefaultUnsafeTypes,
		DefaultHeaders: DefaultHeaders,
		buf:            make([]byte, 0, 512),
		orig:           orig,
	}

	return w, w.Close
}
