package fingerprint

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// This is where the GCE metadata server normally resides. We hardcode the
// "instance" path as well since it's the only one we access here.
const DEFAULT_GCE_URL = "http://169.254.169.254/computeMetadata/v1/instance/"

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
	logger      *log.Logger
	metadataURL string
}

// NewEnvGCEFingerprint is used to create a fingerprint from GCE metadata
func NewEnvGCEFingerprint(logger *log.Logger) Fingerprint {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("GCE_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_GCE_URL
	}

	// assume 2 seconds is enough time for inside GCE network
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: cleanhttp.DefaultTransport(),
	}

	return &EnvGCEFingerprint{
		client:      client,
		logger:      logger,
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
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{
			"Metadata-Flavor": []string{"Google"},
		},
	}

	res, err := f.client.Do(req)
	if err != nil {
		return "", err
	}

	resp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Printf("[ERR]: fingerprint.env_gce: Error reading response body for GCE %s", attribute)
		return "", err
	}

	if res.StatusCode >= 400 {
		return "", ReqError{res.StatusCode}
	}

	return string(resp), nil
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

	if node.Links == nil {
		node.Links = make(map[string]string)
	}

	keys := []string{
		"hostname",
		"id",
		"cpu-platform",
		"scheduling/automatic-restart",
		"scheduling/on-host-maintenance",
	}
	for _, k := range keys {
		value, err := f.Get(k, false)
		if err != nil {
			return false, checkError(err, f.logger, k)
		}

		// assume we want blank entries
		key := strings.Replace(k, "/", ".", -1)
		node.Attributes["platform.gce."+key] = strings.Trim(string(value), "\n")
	}

	// These keys need everything before the final slash removed to be usable.
	keys = []string{
		"machine-type",
		"zone",
	}
	for _, k := range keys {
		value, err := f.Get(k, false)
		if err != nil {
			return false, checkError(err, f.logger, k)
		}

		node.Attributes["platform.gce."+k] = strings.Trim(lastToken(value), "\n")
	}

	// Get internal and external IPs (if they exist)
	value, err := f.Get("network-interfaces/", true)
	var interfaces []GCEMetadataNetworkInterface
	if err := json.Unmarshal([]byte(value), &interfaces); err != nil {
		f.logger.Printf("[WARN] fingerprint.env_gce: Error decoding network interface information: %s", err.Error())
	}

	for _, intf := range interfaces {
		prefix := "platform.gce.network." + lastToken(intf.Network)
		node.Attributes[prefix] = "true"
		node.Attributes[prefix+".ip"] = strings.Trim(intf.Ip, "\n")
		for index, accessConfig := range intf.AccessConfigs {
			node.Attributes[prefix+".external-ip."+strconv.Itoa(index)] = accessConfig.ExternalIp
		}
	}

	var tagList []string
	value, err = f.Get("tags", false)
	if err != nil {
		return false, checkError(err, f.logger, "tags")
	}
	if err := json.Unmarshal([]byte(value), &tagList); err != nil {
		f.logger.Printf("[WARN] fingerprint.env_gce: Error decoding instance tags: %s", err.Error())
	}
	for _, tag := range tagList {
		node.Attributes["platform.gce.tag."+tag] = "true"
	}

	var attrDict map[string]string
	value, err = f.Get("attributes/", true)
	if err != nil {
		return false, checkError(err, f.logger, "attributes/")
	}
	if err := json.Unmarshal([]byte(value), &attrDict); err != nil {
		f.logger.Printf("[WARN] fingerprint.env_gce: Error decoding instance attributes: %s", err.Error())
	}
	for k, v := range attrDict {
		node.Attributes["platform.gce.attr."+k] = strings.Trim(v, "\n")
	}

	// populate Links
	node.Links["gce"] = node.Attributes["platform.gce.id"]

	return true, nil
}

func (f *EnvGCEFingerprint) isGCE() bool {
	// TODO: better way to detect GCE?

	// Query the metadata url for the machine type, to verify we're on GCE
	machineType, err := f.Get("machine-type", false)
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
