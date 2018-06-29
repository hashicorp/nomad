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

type ProvisionerOptions struct {
	Name    string
	Servers int
	Clients int
}

type ClusterInfo struct {
	ID           string
	Name         string
	Servers      []string
	Clients      []string
	NomadClient  *napi.Client
	ConsulClient *capi.Client
	VaultClient  *vapi.Client
}

type Provisioner interface {
	ProvisionCluster(opts ProvisionerOptions) (*ClusterInfo, error)
	DestroyCluster(clusterID string) error
}

var DefaultProvisioner Provisioner = new(singleClusterProvisioner)

type singleClusterProvisioner struct{}

func (p *singleClusterProvisioner) ProvisionCluster(opts ProvisionerOptions) (*ClusterInfo, error) {
	h := md5.New()
	h.Write([]byte(opts.Name))
	info := &ClusterInfo{
		ID:   hex.EncodeToString(h.Sum(nil))[:8],
		Name: opts.Name,
	}

	if len(os.Getenv("NOMAD_ADDR")) == 0 {
		return nil, fmt.Errorf("environment variable NOMAD_ADDR not set")
	}

	nomadClient, err := napi.NewClient(napi.DefaultConfig())
	if err != nil {
		return nil, err
	}
	info.NomadClient = nomadClient

	if len(os.Getenv(capi.HTTPAddrEnvName)) != 0 {
		consulClient, err := capi.NewClient(capi.DefaultConfig())
		if err != nil {
			return nil, err
		}
		info.ConsulClient = consulClient
	}

	if len(os.Getenv(vapi.EnvVaultAddress)) != 0 {
		vaultClient, err := vapi.NewClient(vapi.DefaultConfig())
		if err != nil {
			return nil, err
		}
		info.VaultClient = vaultClient
	}

	return info, err
}

func (p *singleClusterProvisioner) DestroyCluster(_ string) error {
	//Maybe try to GC things based on id?
	return nil
}
