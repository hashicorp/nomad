package provisioning

import "testing"

// ProvisioningTargets is a set of hosts that will get a Nomad and/or Consul
// deployment.
type ProvisioningTargets struct {
	Servers []*ProvisioningTarget `json:"servers"`
	Clients []*ProvisioningTarget `json:"clients"`
}

// ProvisioningTarget is a specific host that will get a Nomad and/or Consul
// deployment.
type ProvisioningTarget struct {
	Deployment Deployment             `json:"deployment"`
	Runner     map[string]interface{} `json:"runner"`
	runner     ProvisioningRunner
}

type Deployment struct {
	// Note: these override each other. NomadLocalBinary > NomadSha > NomadVersion
	NomadLocalBinary string `json:"nomad_local_binary"`
	NomadSha         string `json:"nomad_sha"`
	NomadVersion     string `json:"nomad_version"`

	RemoteBinaryPath string   `json:"remote_binary_path"`
	Platform         string   `json:"platform"`
	Bundles          []Bundle `json:"bundles"`
	Steps            []string `json:"steps"`
}

// Bundle is an arbitrary collection of files to support Nomad, Consul,
// etc. that will be placed as part of provisioning.
type Bundle struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// ProvisioningRunner is the interface for how a Provisioner will connect
// with a target and execute steps. Each target has its own runner.
type ProvisioningRunner interface {
	Open(t *testing.T) error   // start the runner, caching connection info
	Run(string) error          // run one step
	Copy(string, string) error // copy a file
	Close()                    // clean up the runner, call in a defer
}
