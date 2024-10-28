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
	"sync"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
)

type HostVolumeManager struct {
	log            hclog.Logger
	sharedMountDir string

	pluginsLock sync.Mutex
	plugins     map[string]hostVolumePluginFunc
}

func NewHostVolumeManager(sharedMountDir string, logger hclog.Logger) *HostVolumeManager {

	log := logger.Named("host_volumes")

	return &HostVolumeManager{
		log:            log,
		sharedMountDir: sharedMountDir,
		plugins: map[string]hostVolumePluginFunc{
			// TODO: how do we define the external mounter plugins? plugin configs?
			// note that these can't be in the usual go-plugins directory
			"default-mounter": newDefaultHostVolumePluginFn(log, sharedMountDir),
			"example-host-volume": newExternalHostVolumePluginFn(
				log, sharedMountDir, "/opt/nomad/hostvolumeplugins/example-host-volume"),
		},
	}
}

func (hvm *HostVolumeManager) Create(ctx context.Context, req *cstructs.ClientHostVolumeCreateRequest) (*cstructs.ClientHostVolumeCreateResponse, error) {
	hvm.pluginsLock.Lock()
	pluginFn, ok := hvm.plugins[req.PluginID]
	hvm.pluginsLock.Unlock()
	if !ok {
		return nil, fmt.Errorf("no such plugin %q", req.PluginID)
	}

	volID := uuid.Generate()
	pluginResp, err := pluginFn(ctx, volID, req)
	if err != nil {
		return nil, err
	}

	resp := &cstructs.ClientHostVolumeCreateResponse{
		HostPath:      pluginResp.Path,
		CapacityBytes: pluginResp.SizeInMB,
	}

	// TODO: now we need to add it to the node fingerprint!

	return resp, nil
}

type hostVolumePluginFunc func(ctx context.Context, id string, req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginResponse, error)

type hostVolumePluginResponse struct {
	Path     string            `json:"path"`
	SizeInMB int64             `json:"size"`
	Context  map[string]string `json:"context"` // metadata
}

func newExternalHostVolumePluginFn(log hclog.Logger, sharedMountDir, executablePath string) hostVolumePluginFunc {
	return func(ctx context.Context, id string, req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginResponse, error) {

		buf, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}

		log.Trace("external host volume plugin", "req", string(buf))

		stdin := bytes.NewReader(buf)
		targetPath := filepath.Join(sharedMountDir, id)

		cmd := exec.CommandContext(ctx, executablePath, targetPath)
		cmd.Stdin = stdin

		var stderr bytes.Buffer
		cmd.Stderr = io.Writer(&stderr)
		outBuf, err := cmd.Output()
		if err != nil {
			out, _ := io.ReadAll(&stderr)
			return nil, fmt.Errorf("hostvolume plugin failed: %v, %v", err, string(out))
		}

		var pluginResp hostVolumePluginResponse
		err = json.Unmarshal(outBuf, &pluginResp)
		if err != nil {
			return nil, err
		}

		return &pluginResp, nil
	}
}

func newDefaultHostVolumePluginFn(log hclog.Logger, sharedMountDir string) hostVolumePluginFunc {
	return func(ctx context.Context, id string, req *cstructs.ClientHostVolumeCreateRequest) (*hostVolumePluginResponse, error) {

		targetPath := filepath.Join(sharedMountDir, id)
		log.Trace("default host volume plugin", "target_path", targetPath)

		err := os.Mkdir(targetPath, 0o700)
		if err != nil {
			return nil, err
		}

		return &hostVolumePluginResponse{
			Path:     targetPath,
			SizeInMB: 0,
			Context:  map[string]string{},
		}, nil
	}
}
