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
	"regexp"
	"strconv"
	"strings"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// This is where the GCE metadata server normally resides. We hardcode the
	// "instance" path as well since it's the only one we access here.
	DEFAULT_GCE_URL = "http://169.254.169.254/computeMetadata/v1/instance/"

	// GceMetadataTimeout is the timeout used when contacting the GCE metadata
	// service
	GceMetadataTimeout = 2 * time.Second
)

type GCEMetadataNetworkInterface struct {
	AccessConfigs []struct {
		ExternalIp string
		Type       string
	}
	ForwardedIps []string
	Ip           string
	Network      string
}

type ReqError struct {
	StatusCode int
}

func (e ReqError) Error() string {
	return http.StatusText(e.StatusCode)
}

func lastToken(s string) string {
	index := strings.LastIndex(s, "/")
	return s[index+1:]
}

// EnvGCEFingerprint is used to fingerprint GCE metadata
type EnvGCEFingerprint struct {
	StaticFingerprinter
	client      *http.Client
	logger      log.Logger
	metadataURL string
}

// NewEnvGCEFingerprint is used to create a fingerprint from GCE metadata
func NewEnvGCEFingerprint(logger log.Logger) Fingerprint {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("GCE_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_GCE_URL
	}

	// assume 2 seconds is enough time for inside GCE network
	client := &http.Client{
		Timeout:   GceMetadataTimeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	return &EnvGCEFingerprint{
		client:      client,
		logger:      logger.Named("env_gce"),
		metadataURL: metadataURL,
	}
}

func (f *EnvGCEFingerprint) Get(attribute string, recursive bool) (string, error) {
	reqUrl := f.metadataURL + attribute
	if recursive {
		reqUrl = reqUrl + "?recursive=true"
	}

	parsedUrl, err := url.Parse(reqUrl)
	if err != nil {
		return "", err
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    parsedUrl,
		Header: http.Header{
			"Metadata-Flavor": []string{"Google"},
			"User-Agent":      []string{useragent.String()},
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
		f.logger.Error("error reading response body for GCE attribute", "attribute", attribute, "error", err)
		return "", err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return "", ReqError{res.StatusCode}
	}

	return string(resp), nil
}

func checkError(err error, logger log.Logger, desc string) error {
	// If it's a URL error, assume we're not actually in an GCE environment.
	// To the outer layers, this isn't an error so return nil.
	if _, ok := err.(*url.Error); ok {
		logger.Debug("error querying GCE attribute; skipping", "attribute", desc)
		return nil
	}
	// Otherwise pass the error through.
	return err
}

func (f *EnvGCEFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	cfg := req.Config

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		f.client.Timeout = 1 * time.Millisecond
	}

	if !f.isGCE() {
		return nil
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]bool{
		"hostname":                       true,
		"id":                             true,
		"cpu-platform":                   false,
		"scheduling/automatic-restart":   false,
		"scheduling/on-host-maintenance": false,
	}

	for k, unique := range keys {
		value, err := f.Get(k, false)
		if err != nil {
			return checkError(err, f.logger, k)
		}

		// assume we want blank entries
		key := "platform.gce." + strings.ReplaceAll(k, "/", ".")
		if unique {
			key = structs.UniqueNamespace(key)
		}
		resp.AddAttribute(key, strings.Trim(value, "\n"))
	}

	// These keys need everything before the final slash removed to be usable.
	keys = map[string]bool{
		"machine-type": false,
		"zone":         false,
	}
	for k, unique := range keys {
		value, err := f.Get(k, false)
		if err != nil {
			return checkError(err, f.logger, k)
		}

		key := "platform.gce." + k
		if unique {
			key = structs.UniqueNamespace(key)
		}
		resp.AddAttribute(key, strings.Trim(lastToken(value), "\n"))
	}

	// Get internal and external IPs (if they exist)
	value, err := f.Get("network-interfaces/", true)
	if err != nil {
		f.logger.Warn("error retrieving network interface information", "error", err)
	} else {

		var interfaces []GCEMetadataNetworkInterface
		if err := json.Unmarshal([]byte(value), &interfaces); err != nil {
			f.logger.Warn("error decoding network interface information", "error", err)
		}

		for _, intf := range interfaces {
			prefix := "platform.gce.network." + lastToken(intf.Network)
			uniquePrefix := "unique." + prefix
			resp.AddAttribute(prefix, "true")
			resp.AddAttribute(uniquePrefix+".ip", strings.Trim(intf.Ip, "\n"))
			for index, accessConfig := range intf.AccessConfigs {
				resp.AddAttribute(uniquePrefix+".external-ip."+strconv.Itoa(index), accessConfig.ExternalIp)
			}
		}
	}

	var tagList []string
	value, err = f.Get("tags", false)
	if err != nil {
		return checkError(err, f.logger, "tags")
	}
	if err := json.Unmarshal([]byte(value), &tagList); err != nil {
		f.logger.Warn("error decoding instance tags", "error", err)
	}
	for _, tag := range tagList {
		attr := "platform.gce.tag."
		var key string

		// If the tag is namespaced as unique, we strip it from the tag and
		// prepend to the whole attribute.
		if structs.IsUniqueNamespace(tag) {
			tag = strings.TrimPrefix(tag, structs.NodeUniqueNamespace)
			key = fmt.Sprintf("%s%s%s", structs.NodeUniqueNamespace, attr, tag)
		} else {
			key = fmt.Sprintf("%s%s", attr, tag)
		}

		resp.AddAttribute(key, "true")
	}

	var attrDict map[string]string
	value, err = f.Get("attributes/", true)
	if err != nil {
		return checkError(err, f.logger, "attributes/")
	}
	if err := json.Unmarshal([]byte(value), &attrDict); err != nil {
		f.logger.Warn("error decoding instance attributes", "error", err)
	}
	for k, v := range attrDict {
		attr := "platform.gce.attr."
		var key string

		// If the key is namespaced as unique, we strip it from the
		// key and prepend to the whole attribute.
		if structs.IsUniqueNamespace(k) {
			k = strings.TrimPrefix(k, structs.NodeUniqueNamespace)
			key = fmt.Sprintf("%s%s%s", structs.NodeUniqueNamespace, attr, k)
		} else {
			key = fmt.Sprintf("%s%s", attr, k)
		}

		resp.AddAttribute(key, strings.Trim(v, "\n"))
	}

	// populate Links
	if id, ok := resp.Attributes["unique.platform.gce.id"]; ok {
		resp.AddLink("gce", id)
	}

	resp.Detected = true

	return nil
}

func (f *EnvGCEFingerprint) isGCE() bool {
	// TODO: better way to detect GCE?

	// Query the metadata url for the machine type, to verify we're on GCE
	machineType, err := f.Get("machine-type", false)
	if err != nil {
		if re, ok := err.(ReqError); !ok || re.StatusCode != 404 {
			// If it wasn't a 404 error, print an error message.
			f.logger.Debug("error querying GCE Metadata URL, skipping")
		}
		return false
	}

	match, err := regexp.MatchString("projects/.+/machineTypes/.+", machineType)
	if err != nil || !match {
		return false
	}

	return true
}
