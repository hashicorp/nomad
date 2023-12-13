// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
)

// assert ResponseRecorder implements the http.ResponseWriter interface
var _ http.ResponseWriter = (*ResponseRecorder)(nil)

// ResponseRecorder implements a ResponseWriter which can be written to and
// read from concurrently. For use in testing streaming APIs where
// httptest.ResponseRecorder is unsafe for concurrent access. Uses
// httptest.ResponseRecorder internally and exposes most of the functionality.
type ResponseRecorder struct {
	rr *httptest.ResponseRecorder
	mu sync.Mutex
}

func NewResponseRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		rr: httptest.NewRecorder(),
	}
}

// Flush sets Flushed=true.
func (r *ResponseRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rr.Flush()
}

// Flushed returns true if Flush has been called.
func (r *ResponseRecorder) Flushed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rr.Flushed
}

// Header returns the response headers. Readers should call HeaderMap() to
// avoid races due to the server concurrently mutating headers.
func (r *ResponseRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rr.Header()
}

// HeaderMap returns the HTTP headers written before WriteHeader was called.
func (r *ResponseRecorder) HeaderMap() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rr.HeaderMap
}

// Write to the underlying response buffer. Safe to call concurrent with Read.
func (r *ResponseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rr.Body.Write(p)
}

// WriteHeader sets the response code and freezes the headers returned by
// HeaderMap. Safe to call concurrent with Read and HeaderMap.
func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rr.WriteHeader(statusCode)
}

// Read available response bytes. Safe to call concurrently with Write().
func (r *ResponseRecorder) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rr.Body.Read(p)
}
