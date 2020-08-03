param(
    [string]$Cloud = "aws",
    [string]$Index=0
)

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Consul
Copy-Item -Force `
  -Path "C:\ops\shared\consul\base.json" `
  -Destination "C:\opt\consul.d\base.json"
Copy-Item -Force `
  -Path "C:\ops\shared\consul\retry_$Cloud.json" `
  -Destination "C:\opt\consul.d\retry_$Cloud.json"
New-Service `
  -Name "Consul" `
  -BinaryPathName "C:\opt\consul.exe agent -config-dir C:\opt\consul.d -log-file C:\opt\consul\consul.log" `
  -StartupType "Automatic" `
  -ErrorAction Ignore
Start-Service "Consul"

# Vault
# TODO(tgross): we don't need Vault for clients
# cp "C:\ops\shared\vault\vault.hcl" C:\opt\vault.d\vault.hcl
# sc.exe create "Vault" binPath= "C:\opt\vault.exe" agent -config-dir "C:\opt\vault.d" start= auto

# install config file
New-Item -ItemType "directory" -Path "C:\opt\nomad" -Force
Copy-Item "C:\ops\shared\nomad\client-windows.hcl" `
  -Destination "C:\opt\nomad.d\nomad.hcl" -Force

# Setup Host Volumes
New-Item -ItemType "directory" -Path "C:\tmp\data" -Force

# TODO(tgross): not sure we even support this for Windows?
# Write-Output "Install CNI"
# md C:\opt\cni\bin
# $cni_url = "https://github.com/containernetworking/plugins/releases/download/v0.8.6/cni-plugins-windows-amd64-v0.8.6.tgz"
# Invoke-WebRequest -Uri "$cni_url" -Outfile cni.tgz
# Expand-7Zip -ArchiveFileName .\cni.tgz -TargetPath C:\opt\cni\bin\ -Force

# needed for metrics scraping HTTP API calls to the client
New-NetFirewallRule -DisplayName 'Nomad HTTP Inbound' -Profile @('Public', 'Domain', 'Private') -Direction Inbound -Action Allow -Protocol TCP -LocalPort @('4646')

# idempotently enable as a service
New-Service `
  -Name "Nomad" `
  -BinaryPathName "C:\opt\nomad.exe agent -config C:\opt\nomad.d" `
  -StartupType "Automatic" `
  -ErrorAction Ignore

Start-Service "Nomad"

Write-Output "Nomad started!"
