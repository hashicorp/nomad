package framework

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	vapi "github.com/hashicorp/vault/api"
)

// ProvisionerOptions defines options to be given to the Provisioner when calling
// ProvisionCluster
type ProvisionerOptions struct {
	Name         string
	ExpectConsul bool // If true, fails if a Consul client can't be configured
	ExpectVault  bool // If true, fails if a Vault client can't be configured
}

type ClusterInfo struct {
	ID           string
	Name         string
	NomadClient  *napi.Client
	ConsulClient *capi.Client
	VaultClient  *vapi.Client
}

// Provisioner interface is used by the test framework to provision a Nomad
// cluster for each TestCase
type Provisioner interface {
	ProvisionCluster(opts ProvisionerOptions) (*ClusterInfo, error)
	DestroyCluster(clusterID string) error
}

// DefaultProvisioner is a noop provisioner that builds clients from environment
// variables according to the respective client configuration
var DefaultProvisioner Provisioner = new(singleClusterProvisioner)

type singleClusterProvisioner struct{}

func (p *singleClusterProvisioner) ProvisionCluster(opts ProvisionerOptions) (*ClusterInfo, error) {
	// Build ID based off given name
	h := md5.New()
	h.Write([]byte(opts.Name))
	info := &ClusterInfo{
		ID:   hex.EncodeToString(h.Sum(nil))[:8],
		Name: opts.Name,
	}

	// Nomad client is required
	if len(os.Getenv("NOMAD_ADDR")) == 0 {
		return nil, fmt.Errorf("environment variable NOMAD_ADDR not set")
	}

	// Build Nomad api client
	nomadClient, err := napi.NewClient(napi.DefaultConfig())
	if err != nil {
		return nil, err
	}
	info.NomadClient = nomadClient

	if len(os.Getenv(capi.HTTPAddrEnvName)) != 0 {
		consulClient, err := capi.NewClient(capi.DefaultConfig())
		if err != nil && opts.ExpectConsul {
			return nil, err
		}
		info.ConsulClient = consulClient
	} else if opts.ExpectConsul {
		return nil, fmt.Errorf("consul client expected but environment variable %s not set",
			capi.HTTPAddrEnvName)
	}

	if len(os.Getenv(vapi.EnvVaultAddress)) != 0 {
		vaultClient, err := vapi.NewClient(vapi.DefaultConfig())
		if err != nil && opts.ExpectVault {
			return nil, err
		}
		info.VaultClient = vaultClient
	} else if opts.ExpectVault {
		return nil, fmt.Errorf("vault client expected but environment variable %s not set",
			vapi.EnvVaultAddress)
	}

	return info, err
}

func (p *singleClusterProvisioner) DestroyCluster(_ string) error {
	//Maybe try to GC things based on id?
	return nil
}
