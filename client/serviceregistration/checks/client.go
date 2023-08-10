// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
	"oss.indeed.com/go/libtime"
)

const (
	// maxTimeoutHTTP is a fail-safe value for the HTTP client, ensuring a Nomad
	// Client does not leak goroutines hanging on to unresponsive endpoints.
	maxTimeoutHTTP = 10 * time.Minute
)

// Checker executes a check given an allocation-specific context, and produces
// a resulting structs.CheckQueryResult
type Checker interface {
	Do(context.Context, *QueryContext, *Query) *structs.CheckQueryResult
}

// New creates a new Checker capable of executing HTTP and TCP checks.
func New(log hclog.Logger) Checker {
	httpClient := cleanhttp.DefaultPooledClient()
	httpClient.Timeout = maxTimeoutHTTP
	return &checker{
		log:        log.Named("checks"),
		httpClient: httpClient,
		clock:      libtime.SystemClock(),
	}
}

type checker struct {
	log        hclog.Logger
	clock      libtime.Clock
	httpClient *http.Client
}

func (c *checker) now() int64 {
	return c.clock.Now().UTC().Unix()
}

// Do will execute the Query given the QueryContext and produce a structs.CheckQueryResult
func (c *checker) Do(ctx context.Context, qc *QueryContext, q *Query) *structs.CheckQueryResult {
	var qr *structs.CheckQueryResult

	timeout, cancel := context.WithTimeout(ctx, q.Timeout)
	defer cancel()

	switch q.Type {
	case "http":
		qr = c.checkHTTP(timeout, qc, q)
	default:
		qr = c.checkTCP(timeout, qc, q)
	}

	qr.ID = qc.ID
	qr.Group = qc.Group
	qr.Task = qc.Task
	qr.Service = qc.Service
	qr.Check = qc.Check
	return qr
}

// resolve the address to use when executing Query given a QueryContext
func address(qc *QueryContext, q *Query) (string, error) {
	mode := q.AddressMode
	if mode == "" { // determine resolution for check address
		if qc.CustomAddress != "" {
			// if the service is using a custom address, enable the check to
			// inherit that custom address
			mode = structs.AddressModeAuto
		} else {
			// otherwise a check defaults to the host address
			mode = structs.AddressModeHost
		}
	}

	label := q.PortLabel
	if label == "" {
		label = qc.ServicePortLabel
	}

	status := qc.NetworkStatus.NetworkStatus()
	addr, port, err := serviceregistration.GetAddress(
		qc.CustomAddress, // custom address
		mode,             // check address mode
		label,            // port label
		qc.Networks,      // allocation networks
		nil,              // driver network (not supported)
		qc.Ports,         // ports
		status,           // allocation network status
	)
	if err != nil {
		return "", err
	}
	if port > 0 {
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return addr, nil
}

func (c *checker) checkTCP(ctx context.Context, qc *QueryContext, q *Query) *structs.CheckQueryResult {
	qr := &structs.CheckQueryResult{
		Mode:      q.Mode,
		Timestamp: c.now(),
		Status:    structs.CheckPending,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	if _, err = new(net.Dialer).DialContext(ctx, "tcp", addr); err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	qr.Output = "nomad: tcp ok"
	qr.Status = structs.CheckSuccess
	return qr
}

func (c *checker) checkHTTP(ctx context.Context, qc *QueryContext, q *Query) *structs.CheckQueryResult {
	qr := &structs.CheckQueryResult{
		Mode:      q.Mode,
		Timestamp: c.now(),
		Status:    structs.CheckPending,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	relative, err := url.Parse(q.Path)
	if err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	base := url.URL{
		Scheme: q.Protocol,
		Host:   addr,
	}
	u := base.ResolveReference(relative).String()

	request, err := http.NewRequest(q.Method, u, nil)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Status = structs.CheckFailure
		return qr
	}

	for header, values := range q.Headers {
		for _, value := range values {
			request.Header.Add(header, value)
		}
	}

	if len(request.Header.Get(useragent.Header)) == 0 {
		request.Header.Set(useragent.Header, useragent.String())
	}

	request.Host = request.Header.Get("Host")
	request.Body = io.NopCloser(strings.NewReader(q.Body))
	request = request.WithContext(ctx)

	result, err := c.httpClient.Do(request)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Status = structs.CheckFailure
		return qr
	}
	defer func() {
		_ = result.Body.Close()
	}()

	// match the result status code to the http status code
	qr.StatusCode = result.StatusCode

	switch {
	case result.StatusCode == http.StatusOK:
		qr.Status = structs.CheckSuccess
		qr.Output = "nomad: http ok"
		return qr
	case result.StatusCode < http.StatusBadRequest:
		qr.Status = structs.CheckSuccess
	default:
		qr.Status = structs.CheckFailure
	}

	// status code was not 200; read the response body and set that as the
	// check result output content
	qr.Output = limitRead(result.Body)

	return qr
}

const (
	// outputSizeLimit is the maximum number of bytes to read and store of an http
	// check output. Set to 3kb which fits in 1 page with room for other fields.
	outputSizeLimit = 3 * 1024
)

func limitRead(r io.Reader) string {
	b := make([]byte, 0, outputSizeLimit)
	output := bytes.NewBuffer(b)
	limited := io.LimitReader(r, outputSizeLimit)
	if _, err := io.Copy(output, limited); err != nil {
		return fmt.Sprintf("nomad: %s", err.Error())
	}
	return output.String()
}
