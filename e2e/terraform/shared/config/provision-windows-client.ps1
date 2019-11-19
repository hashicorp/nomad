param(
    [string]$Cloud = "aws",
    [string]$NomadSha = "",
    [string]$Index=0
)

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Consul
cp "C:\ops\shared\consul\base.json" "C:\opt\consul.d\base.json"
cp "C:\ops\shared\consul\retry_$Cloud.json" "C:\opt\consul.d\retry_$Cloud.json"
sc.exe create "Consul" binPath= "C:\opt\consul.exe agent -config-dir C:\opt\consul.d -log-file C:\opt\consul\consul.log" start= auto
sc.exe start "Consul"

# Vault
# TODO(tgross): we don't need Vault for clients
# cp "C:\ops\shared\vault\vault.hcl" C:\opt\vault.d\vault.hcl
# sc.exe create "Vault" binPath= "C:\opt\vault.exe" agent -config-dir "C:\opt\vault.d" start= auto

# Nomad

md C:\opt\nomad

Read-S3Object `
  -BucketName nomad-team-test-binary `
  -Key "builds-oss/nomad_windows_amd64_$NomadSha.zip" `
  -File .\nomad.zip

Expand-Archive .\nomad.zip .\
rm C:\opt\nomad.exe
mv .\pkg\windows_amd64\nomad.exe C:\opt\nomad.exe

# install config file
cp "C:\ops\shared\nomad\client-windows.hcl" "C:\opt\nomad.d\nomad.hcl"

# Setup Host Volumes
md C:\tmp\data

# TODO(tgross): not sure we even support this for Windows?
# Write-Output "Install CNI"
# md C:\opt\cni\bin
# $cni_url = "https://github.com/containernetworking/plugins/releases/download/v0.8.2/cni-plugins-windows-amd64-v0.8.2.tgz"
# Invoke-WebRequest -Uri "$cni_url" -Outfile cni.tgz
# Expand-7Zip -ArchiveFileName .\cni.tgz -TargetPath C:\opt\cni\bin\

# enable as a service
sc.exe create "Nomad" binPath= "C:\opt\nomad.exe agent -config C:\opt\nomad.d" start= auto
sc.exe start "Nomad"
