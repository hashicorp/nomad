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
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/mitchellh/mapstructure"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
)

const (
	// environment variables for external plugins

	EnvOperation   = "DHV_OPERATION"
	EnvVolumesDir  = "DHV_VOLUMES_DIR"
	EnvPluginDir   = "DHV_PLUGIN_DIR"
	EnvCreatedPath = "DHV_CREATED_PATH"
	EnvNamespace   = "DHV_NAMESPACE"
	EnvVolumeName  = "DHV_VOLUME_NAME"
	EnvVolumeID    = "DHV_VOLUME_ID"
	EnvNodeID      = "DHV_NODE_ID"
	EnvNodePool    = "DHV_NODE_POOL"
	EnvCapacityMin = "DHV_CAPACITY_MIN_BYTES"
	EnvCapacityMax = "DHV_CAPACITY_MAX_BYTES"
	EnvParameters  = "DHV_PARAMETERS"

	// DefaultMkdirFileMode sets the mode of directories created by the
	// "mkdir" built-in plugin to "0700"
	DefaultMkdirFileMode os.FileMode = 0o700
)

// HostVolumePlugin manages the lifecycle of volumes.
type HostVolumePlugin interface {
	Fingerprint(ctx context.Context) (*PluginFingerprint, error)
	Create(ctx context.Context, req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error)
	Delete(ctx context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error
}

// PluginFingerprint gets set on the node for volume scheduling.
// Plugins are expected to respond to 'fingerprint' calls with json that
// unmarshals to this struct.
type PluginFingerprint struct {
	Version *version.Version `json:"version"`
}

// HostVolumePluginCreateResponse returns values to the server that may be shown
// to the user or stored in server state. Plugins are expected to respond to
// 'create' calls with json that unmarshals to this struct.
type HostVolumePluginCreateResponse struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"bytes"`
	Error     string `json:"error"`
}

// HostVolumePluginDeleteResponse returns values to the server that may be shown
// to the user or stored in server state. Plugins are expected to respond to
// 'delete' calls with json that unmarshals to this struct.
type HostVolumePluginDeleteResponse struct {
	Error string `json:"error"`
}

const HostVolumePluginMkdirID = "mkdir"
const HostVolumePluginMkdirVersion = "0.0.1"

// HostVolumePluginMkdirParams represents the parameters{} that the "mkdir"
// plugin will accept.
type HostVolumePluginMkdirParams struct {
	Uid  int         `mapstructure:"uid"`
	Gid  int         `mapstructure:"gid"`
	Mode os.FileMode `mapstructure:"mode"`
}

var _ HostVolumePlugin = &HostVolumePluginMkdir{}

// HostVolumePluginMkdir is a plugin that creates a directory within the
// specified VolumesDir. It is built-in to Nomad, so is always available.
type HostVolumePluginMkdir struct {
	ID         string
	VolumesDir string

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

	path := filepath.Join(p.VolumesDir, req.ID)
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
		log.Debug("error with path", "error", err)
		return nil, err
	}

	params, err := decodeMkdirParams(req.Parameters)
	if err != nil {
		log.Error("error with parameters", "error", err)
		return nil, err
	}

	err = os.MkdirAll(path, params.Mode)
	if err != nil {
		log.Error("error creating directory", "error", err)
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	// Chown note: A uid or gid of -1 means to not change that value.
	if err = os.Chown(path, params.Uid, params.Gid); err != nil {
		log.Error("error changing owner/group", "error", err, "uid", params.Uid, "gid", params.Gid)
		return nil, fmt.Errorf("error changing owner/group: %w", err)
	}

	log.Debug("plugin ran successfully")
	return resp, nil
}

func decodeMkdirParams(in map[string]string) (HostVolumePluginMkdirParams, error) {
	var out HostVolumePluginMkdirParams

	var meta mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &out,
		Metadata:         &meta,
		ErrorUnused:      true, // error on unexpected config fields
		WeaklyTypedInput: true, // convert strings to target types
	})
	if err != nil {
		return out, fmt.Errorf("error creating decoder: %w", err)
	}
	err = decoder.Decode(in)
	if err != nil {
		return out, fmt.Errorf("error decoding: %w", err)
	}

	// defaults
	for _, field := range meta.Unset {
		switch field {
		case "mode":
			out.Mode = DefaultMkdirFileMode
		case "uid":
			out.Uid = -1 // do not change
		case "gid":
			out.Gid = -1 // do not change
		}
	}

	return out, nil
}

func (p *HostVolumePluginMkdir) Delete(_ context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error {
	path := filepath.Join(p.VolumesDir, req.ID)
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

// NewHostVolumePluginExternal returns an external host volume plugin
// if the specified executable exists on disk.
func NewHostVolumePluginExternal(log hclog.Logger,
	pluginDir, filename, volumesDir, nodePool string) (*HostVolumePluginExternal, error) {
	// this should only be called with already-detected executables,
	// but we'll double-check it anyway, so we can provide a tidy error message
	// if it has changed between fingerprinting and execution.
	executable := filepath.Join(pluginDir, filename)
	f, err := os.Stat(executable)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrPluginNotExists, filename)
		}
		return nil, err
	}
	if !helper.IsExecutable(f) {
		return nil, fmt.Errorf("%w: %q", ErrPluginNotExecutable, filename)
	}
	return &HostVolumePluginExternal{
		ID:         filename,
		Executable: executable,
		VolumesDir: volumesDir,
		PluginDir:  pluginDir,
		NodePool:   nodePool,
		log:        log,
	}, nil
}

// HostVolumePluginExternal calls an executable on disk. All operations
// *must* be idempotent, and safe to be called concurrently per volume.
// For each call, the executable's stdout and stderr may be logged, so plugin
// authors should not include any sensitive information in their plugin outputs.
type HostVolumePluginExternal struct {
	ID         string
	Executable string
	VolumesDir string
	PluginDir  string
	NodePool   string

	log hclog.Logger
}

// Fingerprint calls the executable with the following parameters:
// arguments: $1=fingerprint
// environment:
// - DHV_OPERATION=fingerprint
//
// Response should be valid JSON on stdout, with a "version" key, e.g.:
// {"version": "0.0.1"}
// The version value should be a valid version number as allowed by
// version.NewVersion()
//
// Must complete within 5 seconds
func (p *HostVolumePluginExternal) Fingerprint(ctx context.Context) (*PluginFingerprint, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Executable, "fingerprint")
	cmd.Env = []string{EnvOperation + "=fingerprint"}
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

// Create calls the executable with the following parameters:
// arguments: $1=create
// environment:
// - DHV_OPERATION=create
// - DHV_VOLUMES_DIR={directory to put the volume in}
// - DHV_PLUGIN_DIR={path to directory containing plugins}
// - DHV_NAMESPACE={volume namespace}
// - DHV_VOLUME_NAME={name from the volume specification}
// - DHV_VOLUME_ID={volume ID generated by Nomad}
// - DHV_NODE_ID={Nomad node ID}
// - DHV_NODE_POOL={Nomad node pool}
// - DHV_CAPACITY_MIN_BYTES={capacity_min from the volume spec, expressed in bytes}
// - DHV_CAPACITY_MAX_BYTES={capacity_max from the volume spec, expressed in bytes}
// - DHV_PARAMETERS={stringified json of parameters from the volume spec}
//
// Response should be valid JSON on stdout with "path" and "bytes", e.g.:
// {"path": "/path/that/was/created", "bytes": 50000000}
// "path" must be provided to confirm the requested path is what was
// created by the plugin. "bytes" is the actual size of the volume created
// by the plugin; if excluded, it will default to 0.
//
// Must complete within 60 seconds (timeout on RPC)
func (p *HostVolumePluginExternal) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error) {

	params, err := json.Marshal(req.Parameters)
	if err != nil {
		// should never happen; req.Parameters is a simple map[string]string
		return nil, fmt.Errorf("error marshaling volume pramaters: %w", err)
	}
	envVars := []string{
		fmt.Sprintf("%s=%s", EnvOperation, "create"),
		fmt.Sprintf("%s=%s", EnvVolumesDir, p.VolumesDir),
		fmt.Sprintf("%s=%s", EnvPluginDir, p.PluginDir),
		fmt.Sprintf("%s=%s", EnvNodePool, p.NodePool),
		// values from volume spec
		fmt.Sprintf("%s=%s", EnvNamespace, req.Namespace),
		fmt.Sprintf("%s=%s", EnvVolumeName, req.Name),
		fmt.Sprintf("%s=%s", EnvVolumeID, req.ID),
		fmt.Sprintf("%s=%d", EnvCapacityMin, req.RequestedCapacityMinBytes),
		fmt.Sprintf("%s=%d", EnvCapacityMax, req.RequestedCapacityMaxBytes),
		fmt.Sprintf("%s=%s", EnvNodeID, req.NodeID),
		fmt.Sprintf("%s=%s", EnvParameters, params),
	}

	var pluginResp HostVolumePluginCreateResponse
	log := p.log.With("volume_name", req.Name, "volume_id", req.ID)
	stdout, _, err := p.runPlugin(ctx, log, "create", envVars)
	if err != nil {
		jsonErr := json.Unmarshal(stdout, &pluginResp)
		if jsonErr != nil {
			// if we got an error, we can't actually count on getting JSON, so
			// optimistically look for it and return the original error
			// otherwise
			return nil, fmt.Errorf(
				"error creating volume %q with plugin %q: %w", req.ID, p.ID, err)
		}
		return nil, fmt.Errorf("error creating volume %q with plugin %q: %w: %s",
			req.ID, p.ID, err, pluginResp.Error)
	}
	err = json.Unmarshal(stdout, &pluginResp)
	if err != nil {
		// note: if a plugin does not return valid json, a volume may be
		// created without any respective state in Nomad, since we return
		// an error here after the plugin has done who-knows-what.
		return nil, err
	}
	return &pluginResp, nil
}

// Delete calls the executable with the following parameters:
// arguments: $1=delete
// environment:
// - DHV_OPERATION=delete
// - DHV_CREATED_PATH={path that `create` returned}
// - DHV_VOLUMES_DIR={directory that volumes should be put in}
// - DHV_PLUGIN_DIR={path to directory containing plugins}
// - DHV_NAMESPACE={volume namespace}
// - DHV_VOLUME_NAME={name from the volume specification}
// - DHV_VOLUME_ID={volume ID generated by Nomad}
// - DHV_NODE_ID={Nomad node ID}
// - DHV_NODE_POOL={Nomad node pool}
// - DHV_PARAMETERS={stringified json of parameters from the volume spec}
//
// Response on stdout is discarded.
//
// Must complete within 60 seconds (timeout on RPC)
func (p *HostVolumePluginExternal) Delete(ctx context.Context,
	req *cstructs.ClientHostVolumeDeleteRequest) error {

	params, err := json.Marshal(req.Parameters)
	if err != nil {
		// should never happen; req.Parameters is a simple map[string]string
		return fmt.Errorf("error marshaling volume pramaters: %w", err)
	}
	envVars := []string{
		fmt.Sprintf("%s=%s", EnvOperation, "delete"),
		fmt.Sprintf("%s=%s", EnvVolumesDir, p.VolumesDir),
		fmt.Sprintf("%s=%s", EnvPluginDir, p.PluginDir),
		fmt.Sprintf("%s=%s", EnvNodePool, p.NodePool),
		// from create response
		fmt.Sprintf("%s=%s", EnvCreatedPath, req.HostPath),
		// values from volume spec
		fmt.Sprintf("%s=%s", EnvNamespace, req.Namespace),
		fmt.Sprintf("%s=%s", EnvVolumeName, req.Name),
		fmt.Sprintf("%s=%s", EnvVolumeID, req.ID),
		fmt.Sprintf("%s=%s", EnvNodeID, req.NodeID),
		fmt.Sprintf("%s=%s", EnvParameters, params),
	}

	log := p.log.With("volume_name", req.Name, "volume_id", req.ID)
	stdout, _, err := p.runPlugin(ctx, log, "delete", envVars)
	if err != nil {
		var pluginResp HostVolumePluginDeleteResponse
		jsonErr := json.Unmarshal(stdout, &pluginResp)
		if jsonErr != nil {
			// if we got an error, we can't actually count on getting JSON, so
			// optimistically look for it and return the original error
			// otherwise
			return fmt.Errorf("error reading plugin response when deleting volume %q with plugin %q: original error: %w", req.ID, p.ID, err)
		}
		return fmt.Errorf("error deleting volume %q with plugin %q: %w: %s",
			req.ID, p.ID, err, pluginResp.Error)
	}

	return nil
}

// runPlugin executes the... executable
func (p *HostVolumePluginExternal) runPlugin(ctx context.Context, log hclog.Logger,
	op string, env []string) (stdout, stderr []byte, err error) {

	log = log.With("operation", op)
	log.Debug("running plugin")

	// set up plugin execution
	cmd := exec.CommandContext(ctx, p.Executable, op)
	cmd.Env = env

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

// runCommand executes the provided Cmd and captures stdout and stderr.
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
