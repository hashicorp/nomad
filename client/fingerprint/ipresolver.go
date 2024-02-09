package fingerprint

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/useragent"
)

type IpResolverResponse struct {
	Ip string `json:"ip"`
}

type IpResolverFingerprint struct {
	StaticFingerprinter
	logger     log.Logger
	httpClient *http.Client
}

func NewIpResolverFingerprint(logger log.Logger) Fingerprint {
	return &IpResolverFingerprint{
		logger: logger.Named("ipresolver"),
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: cleanhttp.DefaultTransport(),
		},
	}
}

func (f *IpResolverFingerprint) Fingerprint(fingerprintRequest *FingerprintRequest, fingerprintResponse *FingerprintResponse) error {
	f.logger.Info("loaded configuration", "resolver endpoint", fingerprintRequest.Config.IpResolverEndpoint)

	if fingerprintRequest.Config.IpResolverEndpoint == "" {
		return nil // If no endpoint set, do not resolve IP.
	}

	// Parse URL
	parsedURL, err := url.Parse(fingerprintRequest.Config.IpResolverEndpoint)
	if err != nil {
		return err
	}

	// Prepare HTTP request
	httpRequest := &http.Request{
		Method: "GET",
		URL:    parsedURL,
		Header: http.Header{
			"User-Agent": []string{useragent.String()},
		},
	}

	// Send HTTP request
	httpResponse, err := f.httpClient.Do(httpRequest)
	if err != nil {
		f.logger.Debug("no response from remote", "error", err)
		return err
	} else if httpResponse.StatusCode != http.StatusOK {
		f.logger.Debug("response was not HTTP 200 OK", "http status code", httpResponse.StatusCode)
		return err
	}

	// Wait for response
	httpResponseBody, err := ioutil.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		f.logger.Error("http stream error", "error", err)
		return err
	}

	// Parse the response (json)
	var jsonResponse IpResolverResponse
	httpResponseBodyAsString := strings.TrimSpace(string(httpResponseBody))
	json.Unmarshal([]byte(httpResponseBodyAsString), &jsonResponse)

	if jsonResponse.Ip != "" {
		fingerprintResponse.AddAttribute("ip-resolver.public-ipv4", jsonResponse.Ip)
		fingerprintResponse.Detected = true
	}
	return nil
}

func (f *IpResolverFingerprint) Periodic() (bool, time.Duration) {
	return true, 60 * time.Second
}