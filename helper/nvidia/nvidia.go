package nvidia

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config represents controls how GPUs are available to the container.
// For more details, check [`nvidia-container-runtime`` docs`](https://github.com/nvidia/nvidia-container-runtime#environment-variables-oci-spec)
type Config struct {
	// Devices controls which GPUs will be made accessible inside the container.
	// Defaults to none
	// Valid values are "all", "none", or a comma-separated list of GPU UUI(s) or index(es)
	Devices []string

	// Capabilities controls which driver libraries/binaries will be mounted inside the container
	// Defaults to `utility`
	Capabilities []string

	// Requirements define constraints on the configurations supported by the container
	Requirements []string
}

const nvidiaCLI = "nvidia-container-cli"

func ConfigureContainer(rootfs string, cfg Config) error {
	args, err := cliConfigureArgs(rootfs, cfg)
	if err != nil {
		return err
	}

	cmd := exec.Command(nvidiaCLI, args...)
	return cmd.Run()
}

func cliConfigureArgs(rootfs string, cfg Config) ([]string, error) {
	rootfsAbs, err := filepath.Abs(rootfs)
	if err != nil {
		return nil, fmt.Errorf("failed to find rootfs %s: %v", rootfs, err)
	}

	args := []string{
		// always load kernel modules just in case
		"--load-kmods",
		"configure",
	}

	if len(cfg.Devices) > 0 {
		args = append(args, fmt.Sprintf("--devices=%s", strings.Join(cfg.Devices, ",")))
	} else {
		args = append(args, "--devices=none")
	}

	for _, c := range cfg.Capabilities {
		args = append(args, fmt.Sprintf("--%s", c))
	}

	for _, r := range cfg.Requirements {
		args = append(args, fmt.Sprintf("--require=%s", r))
	}

	args = append(args, rootfsAbs)
	return args, nil
}
