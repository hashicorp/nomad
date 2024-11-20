// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
)

type HostVolumePlugin interface {
	Version(ctx context.Context) (string, error)
	Create(ctx context.Context, req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginCreateResponse, error)
	Delete(ctx context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error
	// db TODO(1.10.0): update? resize? ??
}

type hostVolumePluginCreateResponse struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"bytes"`
	Context   map[string]string `json:"context"` // metadata
}

var _ HostVolumePlugin = &HostVolumePluginMkdir{}

type HostVolumePluginMkdir struct {
	ID         string
	TargetPath string

	log hclog.Logger
}

func (p *HostVolumePluginMkdir) Version(_ context.Context) (string, error) {
	return "0.0.1", nil
}

func (p *HostVolumePluginMkdir) Create(_ context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginCreateResponse, error) {

	path := filepath.Join(p.TargetPath, req.ID)
	p.log.Debug("CREATE: default host volume plugin", "target_path", path)

	err := os.Mkdir(path, 0o700)
	if err != nil {
		return nil, err
	}

	return &hostVolumePluginCreateResponse{
		Path:      path,
		SizeBytes: 0,
		Context:   map[string]string{},
	}, nil
}

func (p *HostVolumePluginMkdir) Delete(_ context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error {
	path := filepath.Join(p.TargetPath, req.ID)
	p.log.Debug("DELETE: default host volume plugin", "target_path", path)
	return os.RemoveAll(path)
}

var _ HostVolumePlugin = &HostVolumePluginExternal{}

type HostVolumePluginExternal struct {
	ID         string
	Executable string
	TargetPath string

	log hclog.Logger
}

func (p *HostVolumePluginExternal) Version(_ context.Context) (string, error) {
	return "0.0.1", nil // db TODO(1.10.0): call the plugin, use in fingerprint
}

func (p *HostVolumePluginExternal) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginCreateResponse, error) {

	params, err := json.Marshal(req.Parameters) // db TODO(1.10.0): if this is nil, then PARAMETERS env will be "null"
	if err != nil {
		return nil, fmt.Errorf("error marshaling volume pramaters: %w", err)
	}
	envVars := []string{
		"NODE_ID=" + req.NodeID,
		"VOLUME_NAME=" + req.Name,
		fmt.Sprintf("CAPACITY_MIN_BYTES=%d", req.RequestedCapacityMinBytes),
		fmt.Sprintf("CAPACITY_MAX_BYTES=%d", req.RequestedCapacityMaxBytes),
		"PARAMETERS=" + string(params),
	}

	stdout, _, err := p.runPlugin(ctx, "create", req.ID, envVars)
	if err != nil {
		return nil, fmt.Errorf("error creating volume %q with plugin %q: %w", req.ID, req.PluginID, err)
	}

	var pluginResp hostVolumePluginCreateResponse
	err = json.Unmarshal(stdout, &pluginResp)
	if err != nil {
		return nil, err
	}
	return &pluginResp, nil
}

func (p *HostVolumePluginExternal) Delete(ctx context.Context,
	req *cstructs.ClientHostVolumeDeleteRequest) error {

	params, err := json.Marshal(req.Parameters)
	if err != nil {
		return fmt.Errorf("error marshaling volume pramaters: %w", err)
	}
	envVars := []string{
		"PARAMETERS=" + string(params),
	}

	_, _, err = p.runPlugin(ctx, "delete", req.ID, envVars)
	if err != nil {
		return fmt.Errorf("error deleting volume %q with plugin %q: %w", req.ID, req.PluginID, err)
	}
	return nil
}

func (p *HostVolumePluginExternal) runPlugin(ctx context.Context,
	op, volID string, env []string) (stdout, stderr []byte, err error) {

	log := p.log.With(
		"operation", op,
		"volume_id", volID,
	)
	log.Debug("running plugin")

	// set up plugin execution
	path := filepath.Join(p.TargetPath, volID)
	cmd := exec.CommandContext(ctx, p.Executable, op, path)

	cmd.Env = append([]string{
		"OPERATION=" + op,
		"HOST_PATH=" + path,
	}, env...)

	var errBuf bytes.Buffer
	cmd.Stderr = io.Writer(&errBuf) // db TODO(1.10.0): maybe a better way to capture stderr?

	// run the command and capture output
	mErr := &multierror.Error{}
	stdout, err = cmd.Output()
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}
	stderr, err = io.ReadAll(&errBuf)
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}

	log = log.With(
		"stdout", string(stdout),
		"stderr", string(stderr),
	)
	if mErr.ErrorOrNil() != nil {
		err = helper.FlattenMultierror(mErr)
		log.Debug("error with plugin", "error", err)
		return stdout, stderr, err
	}
	log.Debug("plugin ran successfully")
	return stdout, stderr, nil
}
