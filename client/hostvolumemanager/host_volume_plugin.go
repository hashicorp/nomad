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
	"github.com/hashicorp/go-version"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
)

type HostVolumePlugin interface {
	Fingerprint(ctx context.Context) (*PluginFingerprint, error)
	Create(ctx context.Context, req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error)
	Delete(ctx context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error
}

type PluginFingerprint struct {
	Version *version.Version `json:"version"`
}

type HostVolumePluginCreateResponse struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"bytes"`
}

const HostVolumePluginMkdirID = "mkdir"
const HostVolumePluginMkdirVersion = "0.0.1"

var _ HostVolumePlugin = &HostVolumePluginMkdir{}

type HostVolumePluginMkdir struct {
	ID         string
	TargetPath string

	log hclog.Logger
}

func (p *HostVolumePluginMkdir) Fingerprint(_ context.Context) (*PluginFingerprint, error) {
	v, err := version.NewVersion(HostVolumePluginMkdirVersion)
	return &PluginFingerprint{
		Version: v,
	}, err
}

func (p *HostVolumePluginMkdir) Create(_ context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error) {

	path := filepath.Join(p.TargetPath, req.ID)
	log := p.log.With(
		"operation", "create",
		"volume_id", req.ID,
		"path", path)
	log.Debug("running plugin")

	resp := &HostVolumePluginCreateResponse{
		Path: path,
		// "mkdir" volumes, being simple directories, have unrestricted size
		SizeBytes: 0,
	}

	if _, err := os.Stat(path); err == nil {
		// already exists
		return resp, nil
	} else if !os.IsNotExist(err) {
		// doesn't exist, but some other path error
		log.Debug("error with plugin", "error", err)
		return nil, err
	}

	err := os.Mkdir(path, 0o700)
	if err != nil {
		log.Debug("error with plugin", "error", err)
		return nil, err
	}

	log.Debug("plugin ran successfully")
	return resp, nil
}

func (p *HostVolumePluginMkdir) Delete(_ context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error {
	path := filepath.Join(p.TargetPath, req.ID)
	log := p.log.With(
		"operation", "delete",
		"volume_id", req.ID,
		"path", path)
	log.Debug("running plugin")

	err := os.RemoveAll(path)
	if err != nil {
		log.Debug("error with plugin", "error", err)
		return err
	}

	log.Debug("plugin ran successfully")
	return nil
}

var _ HostVolumePlugin = &HostVolumePluginExternal{}

func NewHostVolumePluginExternal(log hclog.Logger,
	id, executable, targetPath string) (*HostVolumePluginExternal, error) {
	// this should only be called with already-detected executables,
	// but we'll double-check it anyway, so we can provide a tidy error message
	// if it has changed between fingerprinting and execution.
	f, err := os.Stat(executable)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrPluginNotExists, id)
		}
		return nil, err
	}
	if !helper.IsExecutable(f) {
		return nil, fmt.Errorf("%w: %q", ErrPluginNotExecutable, id)
	}
	return &HostVolumePluginExternal{
		ID:         id,
		Executable: executable,
		TargetPath: targetPath,
		log:        log,
	}, nil
}

type HostVolumePluginExternal struct {
	ID         string
	Executable string
	TargetPath string

	log hclog.Logger
}

func (p *HostVolumePluginExternal) Fingerprint(ctx context.Context) (*PluginFingerprint, error) {
	cmd := exec.CommandContext(ctx, p.Executable, "fingerprint")
	cmd.Env = []string{"OPERATION=fingerprint"}
	stdout, stderr, err := runCommand(cmd)
	if err != nil {
		p.log.Debug("error with plugin",
			"operation", "version",
			"stdout", string(stdout),
			"stderr", string(stderr),
			"error", err)
		return nil, fmt.Errorf("error getting version from plugin %q: %w", p.ID, err)
	}
	fprint := &PluginFingerprint{}
	if err := json.Unmarshal(stdout, fprint); err != nil {
		return nil, fmt.Errorf("error parsing fingerprint output as json: %w", err)
	}
	return fprint, nil
}

func (p *HostVolumePluginExternal) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error) {

	params, err := json.Marshal(req.Parameters)
	if err != nil {
		// should never happen; req.Parameters is a simple map[string]string
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
		return nil, fmt.Errorf("error creating volume %q with plugin %q: %w", req.ID, p.ID, err)
	}

	var pluginResp HostVolumePluginCreateResponse
	err = json.Unmarshal(stdout, &pluginResp)
	if err != nil {
		// note: if a plugin does not return valid json, a volume may be
		// created without any respective state in Nomad, since we return
		// an error here after the plugin has done who-knows-what.
		return nil, err
	}
	return &pluginResp, nil
}

func (p *HostVolumePluginExternal) Delete(ctx context.Context,
	req *cstructs.ClientHostVolumeDeleteRequest) error {

	params, err := json.Marshal(req.Parameters)
	if err != nil {
		// should never happen; req.Parameters is a simple map[string]string
		return fmt.Errorf("error marshaling volume pramaters: %w", err)
	}
	envVars := []string{
		"NODE_ID=" + req.NodeID,
		"PARAMETERS=" + string(params),
	}

	_, _, err = p.runPlugin(ctx, "delete", req.ID, envVars)
	if err != nil {
		return fmt.Errorf("error deleting volume %q with plugin %q: %w", req.ID, p.ID, err)
	}
	return nil
}

func (p *HostVolumePluginExternal) runPlugin(ctx context.Context,
	op, volID string, env []string) (stdout, stderr []byte, err error) {

	path := filepath.Join(p.TargetPath, volID)
	log := p.log.With(
		"operation", op,
		"volume_id", volID,
		"path", path)
	log.Debug("running plugin")

	// set up plugin execution
	cmd := exec.CommandContext(ctx, p.Executable, op, path)

	cmd.Env = append([]string{
		"OPERATION=" + op,
		"HOST_PATH=" + path,
	}, env...)

	stdout, stderr, err = runCommand(cmd)

	log = log.With(
		"stdout", string(stdout),
		"stderr", string(stderr),
	)
	if err != nil {
		log.Debug("error with plugin", "error", err)
		return stdout, stderr, err
	}
	log.Debug("plugin ran successfully")
	return stdout, stderr, nil
}

func runCommand(cmd *exec.Cmd) (stdout, stderr []byte, err error) {
	var errBuf bytes.Buffer
	cmd.Stderr = io.Writer(&errBuf)
	mErr := &multierror.Error{}
	stdout, err = cmd.Output()
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}
	stderr, err = io.ReadAll(&errBuf)
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}
	return stdout, stderr, helper.FlattenMultierror(mErr.ErrorOrNil())
}
