package provisioning

import (
	"fmt"
	"log"
)

// ProvisionerConfigVagrant targets a single-node Vagrant environment.
func ProvisionerConfigVagrant(config ProvisionerConfig) *ProvisioningTargets {

	if config.NomadVersion == "" && config.NomadSha == "" && config.NomadLocalBinary == "" {
		log.Fatal("cannot run vagrant provisioning without a '-nomad.*' flag set")
		return nil
	}

	// TODO(tgross): need a better way to get the right root path, rather
	// than relying on running at the root of the Nomad source directory.
	keyPath := fmt.Sprintf(
		"./.vagrant/machines/%s/virtualbox/private_key", config.VagrantBox)

	return &ProvisioningTargets{
		Servers: []*ProvisioningTarget{
			{
				Runner: map[string]interface{}{}, // unused
				runner: &SSHRunner{
					Key:  keyPath,
					User: "vagrant",
					Host: "127.0.0.1",
					Port: 2222,
				},
				Deployment: Deployment{
					NomadLocalBinary: config.NomadLocalBinary,
					NomadSha:         config.NomadSha,
					NomadVersion:     config.NomadVersion,
					RemoteBinaryPath: "/opt/gopath/bin/nomad",
					Platform:         "linux_amd64",
					Bundles: []Bundle{
						// TODO(tgross): we need a shared vagrant config
						// and service file for this to work.
						{
							Source:      "./config.hcl",
							Destination: "/home/vagrant/config.hcl",
						},
					},
					Steps: []string{
						"sudo systemctl restart consul",
						"sudo systemctl restart nomad",
					},
				},
			},
		},
	}
}
