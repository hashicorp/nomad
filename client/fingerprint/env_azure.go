// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"encoding/json"
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
	// AzureMetadataURL is where the Azure metadata server normally resides. We hardcode the
	// "instance" path as well since it's the only one we access here.
	AzureMetadataURL = "http://169.254.169.254/metadata/instance/"

	// AzureMetadataAPIVersion is the version used when contacting the Azure metadata
	// services.
	AzureMetadataAPIVersion = "2019-06-04"

	// AzureMetadataTimeout is the timeout used when contacting the Azure metadata
	// services.
	AzureMetadataTimeout = 2 * time.Second
)

type AzureMetadataTag struct {
	Name  string
	Value string
}

type AzureMetadataPair struct {
	path   string
	unique bool
}

// EnvAzureFingerprint is used to fingerprint Azure metadata
type EnvAzureFingerprint struct {
	StaticFingerprinter
	client      *http.Client
	logger      log.Logger
	metadataURL string
}

// NewEnvAzureFingerprint is used to create a fingerprint from Azure metadata
func NewEnvAzureFingerprint(logger log.Logger) Fingerprint {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("AZURE_ENV_URL")
	if metadataURL == "" {
		metadataURL = AzureMetadataURL
	}

	// assume 2 seconds is enough time for inside Azure network
	client := &http.Client{
		Timeout:   AzureMetadataTimeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	return &EnvAzureFingerprint{
		client:      client,
		logger:      logger.Named("env_azure"),
		metadataURL: metadataURL,
	}
}

func (f *EnvAzureFingerprint) Get(attribute string, format string) (string, error) {
	reqURL := f.metadataURL + attribute + fmt.Sprintf("?api-version=%s&format=%s", AzureMetadataAPIVersion, format)
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return "", err
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    parsedURL,
		Header: http.Header{
			"Metadata":   []string{"true"},
			"User-Agent": []string{useragent.String()},
		},
	}

	res, err := f.client.Do(req)
	if err != nil {
		f.logger.Debug("could not read value for attribute", "attribute", attribute, "error", err)
		return "", err
	} else if res.StatusCode != http.StatusOK {
		f.logger.Debug("could not read value for attribute", "attribute", attribute, "resp_code", res.StatusCode)
		return "", err
	}

	resp, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Error("error reading response body for Azure attribute", "attribute", attribute, "error", err)
		return "", err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return "", ReqError{res.StatusCode}
	}

	return string(resp), nil
}

func checkAzureError(err error, logger log.Logger, desc string) error {
	// If it's a URL error, assume we're not actually in an Azure environment.
	// To the outer layers, this isn't an error so return nil.
	if _, ok := err.(*url.Error); ok {
		logger.Debug("error querying Azure attribute; skipping", "attribute", desc)
		return nil
	}
	// Otherwise pass the error through.
	return err
}

func (f *EnvAzureFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	cfg := request.Config

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		f.client.Timeout = 1 * time.Millisecond
	}

	if !f.isAzure() {
		return nil
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]AzureMetadataPair{
		"id":             {unique: true, path: "compute/vmId"},
		"name":           {unique: true, path: "compute/name"}, // name might not be the same as hostname
		"location":       {unique: false, path: "compute/location"},
		"resource-group": {unique: false, path: "compute/resourceGroupName"},
		"scale-set":      {unique: false, path: "compute/vmScaleSetName"},
		"vm-size":        {unique: false, path: "compute/vmSize"},
		"zone":           {unique: false, path: "compute/zone"},
		"local-ipv4":     {unique: true, path: "network/interface/0/ipv4/ipAddress/0/privateIpAddress"},
		"public-ipv4":    {unique: true, path: "network/interface/0/ipv4/ipAddress/0/publicIpAddress"},
		"local-ipv6":     {unique: true, path: "network/interface/0/ipv6/ipAddress/0/privateIpAddress"},
		"public-ipv6":    {unique: true, path: "network/interface/0/ipv6/ipAddress/0/publicIpAddress"},
		"mac":            {unique: true, path: "network/interface/0/macAddress"},
	}

	for k, attr := range keys {
		resp, err := f.Get(attr.path, "text")
		v := strings.TrimSpace(resp)
		if err != nil {
			return checkAzureError(err, f.logger, k)
		} else if v == "" {
			f.logger.Debug("read an empty value", "attribute", k)
			continue
		}

		// assume we want blank entries
		key := "platform.azure." + strings.ReplaceAll(k, "/", ".")
		if attr.unique {
			key = structs.UniqueNamespace(key)
		}
		response.AddAttribute(key, v)
	}

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.azure.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
	}

	var tagList []AzureMetadataTag
	value, err := f.Get("compute/tagsList", "json")
	if err != nil {
		return checkAzureError(err, f.logger, "tags")
	}
	if err := json.Unmarshal([]byte(value), &tagList); err != nil {
		f.logger.Warn("error decoding instance tags", "error", err)
	}
	for _, tag := range tagList {
		attr := "platform.azure.tag."
		var key string

		// If the tag is namespaced as unique, we strip it from the tag and
		// prepend to the whole attribute.
		if structs.IsUniqueNamespace(tag.Name) {
			tag.Name = strings.TrimPrefix(tag.Name, structs.NodeUniqueNamespace)
			key = fmt.Sprintf("%s%s%s", structs.NodeUniqueNamespace, attr, tag.Name)
		} else {
			key = fmt.Sprintf("%s%s", attr, tag.Name)
		}

		response.AddAttribute(key, tag.Value)
	}

	// populate Links
	if id, ok := response.Attributes["unique.platform.azure.id"]; ok {
		response.AddLink("azure", id)
	}

	response.Detected = true
	return nil
}

func (f *EnvAzureFingerprint) isAzure() bool {
	v, err := f.Get("compute/azEnvironment", "text")
	v = strings.TrimSpace(v)
	return err == nil && v != ""
}
