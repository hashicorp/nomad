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

	"github.com/docker/go-metrics"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
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

type SinkCfg struct {
	Address string

	Headers map[string]string

	// HttpClient is the client to use. Default will be used if not provided.
	HttpClient *http.Client
}

func defaultCfg() *SinkCfg {
	cfg := &SinkCfg{
		HttpClient: defaultHttpClient(),
		Headers:    make(map[string]string),
	}
	return cfg
}

type WebhookSink struct {
	client *http.Client
	config SinkCfg

	subscription *Subscription

	metricsLables []metrics.Labels
	l             hclog.Logger
}

func NewWebhookSink(cfg *SinkCfg, broker *EventBroker, subReq *SubscribeRequest) (*WebhookSink, error) {
	defConfig := defaultCfg()

	if cfg.Address == "" {
		return nil, fmt.Errorf("invalid address for websink")
	} else if _, err := url.Parse(cfg.Address); err != nil {
		return nil, fmt.Errorf("invalid address '%s' : %v", cfg.Address, err)
	}

	httpClient := defConfig.HttpClient

	sub, err := broker.Subscribe(subReq)
	if err != nil {
		return nil, fmt.Errorf("configuring webhook sink subscription: %w", err)
	}

	return &WebhookSink{
		client:       httpClient,
		config:       *cfg,
		subscription: sub,
	}, nil
}

func (ws *WebhookSink) Start(ctx context.Context) {
	defer ws.subscription.Unsubscribe()

	// TODO handle reconnect
	for {
		events, err := ws.subscription.Next(ctx)
		if err != nil {
			return
			// TODO handle err
		}
		if len(events.Events) == 0 {
			continue
		}

		if err := ws.send(&events); err != nil {
			ws.l.Error("failed to sending event to webhook", "error", err)
			continue
		}
		metrics.SetGaugeWithLabels([]string{"nomad", "event_broker", "network_sink"})
	}
}

func (ws *WebhookSink) send(e *structs.Events) error {
	req, err := ws.toRequest(e)
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

func (ws *WebhookSink) toRequest(e *structs.Events) (*http.Request, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ws.config.Address, buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	for k, v := range ws.config.Headers {
		req.Header.Add(k, v)
	}

	return req, nil
}
