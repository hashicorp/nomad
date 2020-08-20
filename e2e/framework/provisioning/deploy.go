package provisioning

import (
	"fmt"
	"testing"
)

func deploy(t *testing.T, target *ProvisioningTarget) error {
	platform := target.Deployment.Platform
	if target.Deployment.Platform == "linux_amd64" {
		return deployLinux(t, target)
	} else if target.Deployment.Platform == "windows_amd64" {
		return deployWindows(t, target)
	}
	return fmt.Errorf("invalid deployment platform: %v", platform)
}

func deployLinux(t *testing.T, target *ProvisioningTarget) error {
	var err error
	runner := target.runner
	deployment := target.Deployment

	err = runner.Open(t)
	if err != nil {
		return err
	}
	defer runner.Close()

	if deployment.NomadLocalBinary != "" {
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		err = runner.Copy(
			deployment.NomadLocalBinary,
			deployment.RemoteBinaryPath)
		if err != nil {
			return fmt.Errorf("copying Nomad failed: %v", err)
		}
	} else if deployment.NomadSha != "" {
		script := fmt.Sprintf(
			`/opt/install-nomad --nomad_sha %s --nostart`, deployment.NomadSha)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else if deployment.NomadVersion != "" {
		script := fmt.Sprintf(
			`/opt/install-nomad --nomad_version %s --nostart`, deployment.NomadVersion)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else {
		t.Log("no Nomad deployment specified, falling back to 'step' field.")
	}

	for _, bundle := range deployment.Bundles {
		err = runner.Copy(
			bundle.Source, bundle.Destination)
		if err != nil {
			return fmt.Errorf("copying bundle '%s' failed: %v", bundle.Source, err)
		}
	}

	for _, step := range deployment.Steps {
		err = runner.Run(step)
		if err != nil {
			return fmt.Errorf("deployment step %q failed: %v", step, err)
		}
	}
	return nil
}

func deployWindows(t *testing.T, target *ProvisioningTarget) error {
	var err error
	runner := target.runner
	deployment := target.Deployment

	err = runner.Open(t)
	if err != nil {
		return err
	}
	defer runner.Close()

	runner.Run("Stop-Service Nomad -ErrorAction Ignore; $?")

	if deployment.NomadLocalBinary != "" {
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		err = runner.Copy(
			deployment.NomadLocalBinary,
			deployment.RemoteBinaryPath)
		if err != nil {
			return fmt.Errorf("copying Nomad failed: %v", err)
		}
	} else if deployment.NomadSha != "" {
		script := fmt.Sprintf(
			`C:/opt/install-nomad.ps1 -nomad_sha %s -nostart`, deployment.NomadSha)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else if deployment.NomadVersion != "" {
		script := fmt.Sprintf(
			`C:/opt/install-nomad.ps1 -nomad_version %s -nostart`, deployment.NomadVersion)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else {
		t.Log("no Nomad deployment specified, falling back to 'step' field.")
	}

	for _, bundle := range deployment.Bundles {
		err = runner.Copy(
			bundle.Source, bundle.Destination)
		if err != nil {
			return fmt.Errorf("copying bundle '%s' failed: %v", bundle.Source, err)
		}
	}

	for _, step := range deployment.Steps {
		err = runner.Run(step)
		if err != nil {
			return fmt.Errorf("deployment step %q failed: %v", step, err)
		}
	}
	return nil
}
