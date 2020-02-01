package consulacls

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/e2e/framework/provisioning"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// DefaultTFStateFile is the location of the TF state file, as created for the
// e2e test framework. This file is used to extract the TF serial number, which
// is used to determine whether the consul bootstrap process is necessary or has
// already taken place.
const DefaultTFStateFile = "terraform/terraform.tfstate"

// A Manager is used to manipulate whether Consul ACLs are enabled or disabled.
// Only works with TF provisioned clusters.
type Manager interface {
	// Enable Consul ACLs in the Consul cluster. The Consul ACL master token
	// associated with the Consul cluster is returned.
	//
	// A complete bootstrap process will take place if necessary.
	//
	// Once enabled, Consul ACLs can be disabled with Disable.
	Enable(t *testing.T) string

	// Disable Consul ACLs in the Consul Cluster.
	//
	// Once disabled, Consul ACLs can be re-enabled with Enable.
	Disable(t *testing.T)
}

type tfManager struct {
	serial int
}

func New(tfStateFile string) (*tfManager, error) {
	serial, err := extractSerial(tfStateFile)
	if err != nil {
		return nil, err
	}
	return &tfManager{
		serial: serial,
	}, nil
}

func (m *tfManager) Enable(t *testing.T) string {
	// Create the local script runner that will be used to run the ACL management
	// script, this time with the "enable" sub-command.
	var runner provisioning.LinuxRunner
	err := runner.Open(t)
	require.NoError(t, err)

	// Run the consul ACL bootstrap script, which will store the master token
	// in the deterministic path based on the TF state serial number. If the
	// bootstrap process had already taken place, ACLs will be activated but
	// without going through the bootstrap process again, re-using the already
	// existing Consul ACL master token.
	err = runner.Run(strings.Join([]string{
		"consulacls/consul-acls-manage.sh", "enable",
	}, " "))
	require.NoError(t, err)

	// Read the Consul ACL master token that was generated (or if the token
	// already existed because the bootstrap process had already taken place,
	// that one).
	token, err := m.readToken()
	require.NoError(t, err)
	return token
}

type tfState struct {
	Serial int `json:"serial"`
}

// extractSerial will parse the TF state file looking for the serial number.
func extractSerial(filename string) (int, error) {
	if filename == "" {
		filename = DefaultTFStateFile
	}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, errors.Wrap(err, "failed to extract TF serial")
	}
	var state tfState
	if err := json.Unmarshal(b, &state); err != nil {
		return 0, errors.Wrap(err, "failed to extract TF serial")
	}
	return state.Serial, nil
}

// tokenPath returns the expected path for the Consul ACL master token generated
// by the consul-acls-manage.sh bootstrap script for the current TF serial version.
func (m *tfManager) tokenPath() string {
	return fmt.Sprintf("/tmp/e2e-consul-bootstrap-%d.token", m.serial)
}

func (m *tfManager) readToken() (string, error) {
	b, err := ioutil.ReadFile(m.tokenPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (m *tfManager) Disable(t *testing.T) {
	// Create the local script runner that will be used to run the ACL management
	// script, this time with the "disable" sub-command.
	var runner provisioning.LinuxRunner
	err := runner.Open(t)
	require.NoError(t, err)

	// Run the consul ACL bootstrap script, which will modify the Consul Server
	// ACL policies to disable ACLs, and then restart those agents.
	err = runner.Run(strings.Join([]string{
		"consulacls/consul-acls-manage.sh", "disable",
	}, " "))
	require.NoError(t, err)
}
