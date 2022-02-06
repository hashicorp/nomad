package fingerprint

import (
	"io/ioutil"
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
		Method: "GET",
		URL:    parsedURL,
		Header: http.Header{
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

	resp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Error("error reading response body for DigitalOcean attribute", "attribute", attribute, "error", err)
		return "", err
	}

	if res.StatusCode >= 400 {
		return "", ReqError{res.StatusCode}
	}

	return string(resp), nil
}

func checkDigitalOceanError(err error, logger log.Logger, desc string) error {
	// If it's a URL error, assume we're not actually in an DigitalOcean environment.
	// To the outer layers, this isn't an error so return nil.
	if _, ok := err.(*url.Error); ok {
		logger.Debug("error querying DigitalOcean attribute; skipping", "attribute", desc)
		return nil
	}
	// Otherwise pass the error through.
	return err
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
			return checkDigitalOceanError(err, f.logger, k)
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
