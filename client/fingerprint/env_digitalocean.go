// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// DigitalOceanMetadataURL is where the DigitalOcean metadata api normally resides.
	// https://docs.digitalocean.com/products/droplets/how-to/retrieve-droplet-metadata/#how-to-retrieve-droplet-metadata
	DigitalOceanMetadataURL = "http://169.254.169.254/metadata/v1/"

	// DigitalOceanMetadataTimeout is the timeout used when contacting the DigitalOcean metadata
	// services.
	DigitalOceanMetadataTimeout = 2 * time.Second
)

type DigitalOceanMetadataPair struct {
	path   string
	unique bool
}

// EnvDigitalOceanFingerprint is used to fingerprint DigitalOcean metadata
type EnvDigitalOceanFingerprint struct {
	StaticFingerprinter
	client      *http.Client
	logger      log.Logger
	metadataURL string
}

// NewEnvDigitalOceanFingerprint is used to create a fingerprint from DigitalOcean metadata
func NewEnvDigitalOceanFingerprint(logger log.Logger) Fingerprint {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("DO_ENV_URL")
	if metadataURL == "" {
		metadataURL = DigitalOceanMetadataURL
	}

	// assume 2 seconds is enough time for inside DigitalOcean network
	client := &http.Client{
		Timeout:   DigitalOceanMetadataTimeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	return &EnvDigitalOceanFingerprint{
		client:      client,
		logger:      logger.Named("env_digitalocean"),
		metadataURL: metadataURL,
	}
}

func (f *EnvDigitalOceanFingerprint) Get(attribute string, format string) (string, error) {
	reqURL := f.metadataURL + attribute
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return "", err
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    parsedURL,
		Header: http.Header{
			"User-Agent": []string{useragent.String()},
		},
	}

	res, err := f.client.Do(req)
	if err != nil {
		f.logger.Debug("failed to request metadata", "attribute", attribute, "error", err)
		return "", err
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Error("failed to read metadata", "attribute", attribute, "error", err, "resp_code", res.StatusCode)
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		f.logger.Debug("could not read value for attribute", "attribute", attribute, "resp_code", res.StatusCode)
		return "", fmt.Errorf("error reading attribute %s. digitalocean metadata api returned an error: resp_code: %d, resp_body: %s", attribute, res.StatusCode, body)
	}

	return string(body), nil
}

func (f *EnvDigitalOceanFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	cfg := request.Config

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		f.client.Timeout = 1 * time.Millisecond
	}

	if !f.isDigitalOcean() {
		return nil
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]DigitalOceanMetadataPair{
		"id":           {unique: true, path: "id"},
		"hostname":     {unique: true, path: "hostname"},
		"region":       {unique: false, path: "region"},
		"private-ipv4": {unique: true, path: "interfaces/private/0/ipv4/address"},
		"public-ipv4":  {unique: true, path: "interfaces/public/0/ipv4/address"},
		"private-ipv6": {unique: true, path: "interfaces/private/0/ipv6/address"},
		"public-ipv6":  {unique: true, path: "interfaces/public/0/ipv6/address"},
		"mac":          {unique: true, path: "interfaces/public/0/mac"},
	}

	for k, attr := range keys {
		resp, err := f.Get(attr.path, "text")
		v := strings.TrimSpace(resp)
		if err != nil {
			f.logger.Warn("failed to read attribute", "attribute", k, "error", err)
			continue
		} else if v == "" {
			f.logger.Debug("read an empty value", "attribute", k)
			continue
		}

		// assume we want blank entries
		key := "platform.digitalocean." + strings.ReplaceAll(k, "/", ".")
		if attr.unique {
			key = structs.UniqueNamespace(key)
		}
		response.AddAttribute(key, v)
	}

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.digitalocean.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
	}

	// populate Links
	if id, ok := response.Attributes["unique.platform.digitalocean.id"]; ok {
		response.AddLink("digitalocean", id)
	}

	response.Detected = true
	return nil
}

func (f *EnvDigitalOceanFingerprint) isDigitalOcean() bool {
	v, err := f.Get("region", "text")
	v = strings.TrimSpace(v)
	return err == nil && v != ""
}
