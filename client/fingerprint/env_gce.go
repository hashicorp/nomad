package fingerprint

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const DEFAULT_GCE_URL = "http://169.254.169.254/computeMetadata/v1/instance/"

type GCEMetadataClient struct {
	*http.Client
	logger      *log.Logger
	metadataURL string
}

type ReqError struct {
	StatusCode int
}

func (e ReqError) Error() string {
	return http.StatusText(e.StatusCode)
}

func NewGCEMetadataClient(logger *log.Logger) GCEMetadataClient {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("GCE_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_GCE_URL
	}

	// assume 2 seconds is enough time for inside GCE network
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	return GCEMetadataClient{
		Client:      client,
		logger:      logger,
		metadataURL: metadataURL,
	}
}

func (g GCEMetadataClient) Get(attribute string, recursive bool) (string, error) {
	reqUrl := g.metadataURL + attribute
	if recursive {
		reqUrl = reqUrl + "?recursive=true"
	}

	parsedUrl, err := url.Parse(reqUrl)
	if err != nil {
		return "", err
	}

	req := &http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{
			"Metadata-Flavor": []string{"Google"},
		},
	}

	res, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}

	resp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		g.logger.Printf("[ERR]: fingerprint.env_gce: Error reading response body for GCE %s", attribute)
		return "", err
	}

	if res.StatusCode >= 400 {
		return "", ReqError{res.StatusCode}
	}

	return string(resp), nil
}

// EnvGCEFingerprint is used to fingerprint the CPU
type EnvGCEFingerprint struct {
	logger *log.Logger
}

// NewEnvGCEFingerprint is used to create a CPU fingerprint
func NewEnvGCEFingerprint(logger *log.Logger) Fingerprint {
	f := &EnvGCEFingerprint{logger: logger}
	return f
}

func checkError(err error, logger *log.Logger, desc string) error {
	// If it's a URL error, assume we're not actually in an GCE environment.
	// To the outer layers, this isn't an error so return nil.
	if _, ok := err.(*url.Error); ok {
		logger.Printf("[ERR] fingerprint.env_gce: Error querying GCE " + desc + ", skipping")
		return nil
	}
	// Otherwise pass the error through.
	return err
}

func (f *EnvGCEFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	if !f.isGCE() {
		return false, nil
	}

	// newNetwork is populated and addded to the Nodes resources
	newNetwork := &structs.NetworkResource{
		Device: "eth0",
	}

	if node.Links == nil {
		node.Links = make(map[string]string)
	}

	client := NewGCEMetadataClient(f.logger)

	keys := []string{
		"hostname",
		"id",
	}
	for _, k := range keys {
		value, err := client.Get(k, false)
		if err != nil {
			return false, checkError(err, f.logger, k)
		}

		// assume we want blank entries
		node.Attributes["platform.gce."+k] = strings.Trim(string(value), "\n")
	}

	// These keys need everything before the final slash removed to be usable.
	keys = []string{
		"machine-type",
		"zone",
	}
	for _, k := range keys {
		value, err := client.Get(k, false)
		if err != nil {
			return false, checkError(err, f.logger, k)
		}

		index := strings.LastIndex(value, "/")
		value = value[index+1:]
		node.Attributes["platform.gce."+k] = strings.Trim(string(value), "\n")
	}

	// Get internal and external IP (if it exits)
	value, err := client.Get("network-interfaces/0/ip", false)
	if err != nil {
		return false, checkError(err, f.logger, "ip")
	}
	newNetwork.IP = strings.Trim(value, "\n")
	newNetwork.CIDR = newNetwork.IP + "/32"
	node.Attributes["network.ip-address"] = newNetwork.IP

	value, err = client.Get("network-interfaces/0/access-configs/0/external-ip", false)
	if re, ok := err.(ReqError); err != nil && (!ok || re.StatusCode != 404) {
		return false, checkError(err, f.logger, "external IP")
	}
	value = strings.Trim(value, "\n")
	if len(value) > 0 {
		node.Attributes["platform.gce.external-ip"] = value
	}

	var tagList []string
	value, err = client.Get("tags", false)
	if err != nil {
		return false, checkError(err, f.logger, "tags")
	}
	err = json.Unmarshal([]byte(value), &tagList)
	if err == nil {
		for _, tag := range tagList {
			node.Attributes["platform.gce.tag."+tag] = "true"
		}
	} else {
		f.logger.Printf("[WARN] fingerprint.env_gce: Error decoding instance tags: %s", err.Error())
	}

	var attrDict map[string]string
	value, err = client.Get("attributes/", true)
	if err != nil {
		return false, checkError(err, f.logger, "attributes/")
	}
	err = json.Unmarshal([]byte(value), &attrDict)
	if err == nil {
		for k, v := range attrDict {
			node.Attributes["platform.gce.attr."+k] = strings.Trim(v, "\n")
		}
	} else {
		f.logger.Printf("[WARN] fingerprint.env_gce: Error decoding instance attributes: %s", err.Error())
	}

	// populate Node Network Resources
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}
	node.Resources.Networks = append(node.Resources.Networks, newNetwork)

	// populate Links
	node.Links["gce"] = node.Attributes["platform.gce.id"]

	return true, nil
}

func (f *EnvGCEFingerprint) isGCE() bool {
	// TODO: better way to detect GCE?

	client := NewGCEMetadataClient(f.logger)
	// Query the metadata url for the machine type, to verify we're on GCE
	machineType, err := client.Get("machine-type", false)
	if err != nil {
		if re, ok := err.(ReqError); !ok || re.StatusCode != 404 {
			// If it wasn't a 404 error, print an error message.
			f.logger.Printf("[ERR] fingerprint.env_gce: Error querying GCE Metadata URL, skipping")
		}
		return false
	}

	match, err := regexp.MatchString("projects/.+/machineTypes/.+", machineType)
	if !match {
		return false
	}

	return true
}
