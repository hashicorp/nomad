param([string]$cloud="aws", [string]$nomad_sha="")

# TODO: index is currently unused
#[string]$index=0)

# Consul

cp "C:\ops\shared\consul\consul_client_$cloud.json" C:\opt\consul.d\config.json
sc.exe create "Consul" binPath= "C:\opt\consul.exe" agent -config-dir "C:\opt\consul.d" start= auto
sc.exe start "Consul"

# Vault
# TODO(tgross): we don't need Vault for clients
# cp "C:\ops\shared\vault\vault.hcl" C:\opt\vault.d\vault.hcl
# sc.exe create "Vault" binPath= "C:\opt\vault.exe" agent -config-dir "C:\opt\vault.d" start= auto

# Nomad

md C:\opt\nomad

Read-S3Object `
  -BucketName nomad-team-test-binary `
  -Key "builds-oss/nomad_windows_amd64_$nomad_sha.tar.gz" `
  -File .\nomad.tar.gz
rm C:\opt\nomad.exe
Expand-Archive .\nomad.tar.gz C:\opt\nomad.exe

# install config file
cp "C:\ops\shared\nomad\client-windows.hcl" C:\opt\nomad.d\nomad.hcl

# Setup Host Volumes
md C:\tmp\data

Write-Output "Install CNI"
md C:\opt\cni\bin
$cni_url = "https://github.com/containernetworking/plugins/releases/download/v0.8.2/cni-plugins-windows-amd64-v0.8.2.tgz"
Invoke-WebRequest -Uri "$cni_url" -Outfile cni.tgz
Expand-Archive .\cni.tgz C:\opt\cni\bin

# enable as a service
sc.exe create "Nomad" binPath= "C:\opt\nomad.exe" agent -config-dir "C:\opt\nomad.d" start= auto
sc.exe start "Nomad"
