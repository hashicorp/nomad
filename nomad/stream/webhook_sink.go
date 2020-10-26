package stream

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/nomad/structs"
)

func defaultHttpClient() *http.Client {
	httpClient := cleanhttp.DefaultClient()
	transport := httpClient.Transport.(*http.Transport)
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return httpClient
}

type WebhookSink struct {
	client *http.Client

	Address string
}

func NewWebhookSink(eventSink *structs.EventSink) (*WebhookSink, error) {
	if eventSink.Address == "" {
		return nil, fmt.Errorf("invalid address for websink")
	} else if _, err := url.Parse(eventSink.Address); err != nil {
		return nil, fmt.Errorf("invalid address '%s' : %v", eventSink.Address, err)
	}

	httpClient := defaultHttpClient()

	return &WebhookSink{
		Address: eventSink.Address,
		client:  httpClient,
	}, nil
}

func (ws *WebhookSink) Send(ctx context.Context, e *structs.Events) error {
	req, err := ws.toRequest(ctx, e)
	if err != nil {
		return fmt.Errorf("converting event to request: %w", err)
	}

	err = ws.doRequest(req)
	if err != nil {
		return fmt.Errorf("sending request to webhook %w", err)
	}

	return nil
}

func (ws *WebhookSink) doRequest(req *http.Request) error {
	_, err := ws.client.Do(req)
	if err != nil {
		return err
	}

	return nil

}

func (ws *WebhookSink) toRequest(ctx context.Context, e *structs.Events) (*http.Request, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ws.Address, buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}
