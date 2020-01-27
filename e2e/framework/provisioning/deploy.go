package provisioning

import (
	"fmt"
	"path/filepath"
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
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		s3_url := fmt.Sprintf("s3://nomad-team-test-binary/builds-oss/nomad_%s_%s.tar.gz",
			deployment.Platform, deployment.NomadSha,
		)
		remoteDir := filepath.Dir(deployment.RemoteBinaryPath)
		script := fmt.Sprintf(`aws s3 cp %s nomad.tar.gz
			sudo tar -zxvf nomad.tar.gz -C %s
			sudo chmod 0755 %s
			sudo chown root:root %s`,
			s3_url, remoteDir, deployment.RemoteBinaryPath, deployment.RemoteBinaryPath)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else if deployment.NomadVersion != "" {
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		url := fmt.Sprintf("https://releases.hashicorp.com/nomad/%s/nomad_%s_%s.zip",
			deployment.NomadVersion, deployment.NomadVersion, deployment.Platform,
		)
		remoteDir := filepath.Dir(deployment.RemoteBinaryPath)
		script := fmt.Sprintf(`curl -L --fail -o /tmp/nomad.zip %s
			sudo unzip -o /tmp/nomad.zip -d %s
			sudo chmod 0755 %s
			sudo chown root:root %s`,
			url, remoteDir, deployment.RemoteBinaryPath, deployment.RemoteBinaryPath)
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
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		script := fmt.Sprintf(`
			Read-S3Object -BucketName nomad-team-test-binary -Key "builds-oss/nomad_windows_amd64_%s.zip" -File ./nomad.zip
			Expand-Archive ./nomad.zip ./ -Force
			Remove-Item %s  -ErrorAction Ignore
			Move-Item -Path .\pkg\windows_amd64\nomad.exe -Destination %s -Force`,
			deployment.NomadSha, deployment.RemoteBinaryPath,
			deployment.RemoteBinaryPath)
		err = runner.Run(script)
		if err != nil {
			return err
		}
	} else if deployment.NomadVersion != "" {
		if deployment.RemoteBinaryPath == "" {
			return fmt.Errorf("remote binary path not set")
		}
		url := fmt.Sprintf("https://releases.hashicorp.com/nomad/%s/nomad_%s_%s.zip",
			deployment.NomadVersion, deployment.NomadVersion, deployment.Platform,
		)
		script := fmt.Sprintf(`
			Invoke-WebRequest -Uri "%s" -Outfile /.nomad.zip
			Expand-Archive ./nomad.zip ./ -Force
			Remove-Item %s  -ErrorAction Ignore
			Move-Item -Path .\pkg\windows_amd64\nomad.exe -Destination %s -Force`,
			url, deployment.RemoteBinaryPath, deployment.RemoteBinaryPath)
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
