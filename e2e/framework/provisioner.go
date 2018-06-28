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
	nomadAddr := os.Getenv("NOMAD_ADDR")
	if len(nomadAddr) == 0 {
		return nil, fmt.Errorf("environment variable NOMAD_ADDR not set")
	}

	nomadConfig := napi.DefaultConfig()
	nomadConfig.Address = nomadAddr
	nomadClient, err := napi.NewClient(nomadConfig)
	if err != nil {
		return nil, err
	}

	info.NomadClient = nomadClient

	return info, err
}

func (p *singleClusterProvisioner) DestroyCluster(_ string) error {
	//Maybe try to GC things based on id?
	return nil
}
