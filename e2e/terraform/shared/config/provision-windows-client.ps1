# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Consul
Copy-Item -Force `
  -Path "C:\opt\config\full-cluster\consul\base.json" `
  -Destination "C:\opt\consul.d\base.json"
Copy-Item -Force `
  -Path "C:\opt\config\full-cluster\consul\aws.json" `
  -Destination "C:\opt\consul.d\aws.json"
New-Service `
  -Name "Consul" `
  -BinaryPathName "C:\opt\consul.exe agent -config-dir C:\opt\consul.d -log-file C:\opt\consul\consul.log" `
  -StartupType "Automatic" `
  -ErrorAction Ignore
Start-Service "Consul"

# install config file
New-Item -ItemType "directory" -Path "C:\opt\nomad" -Force
Copy-Item "C:\opt\config\full-cluster\nomad\client-windows\client-windows.hcl" `
  -Destination "C:\opt\nomad.d\nomad.hcl" -Force

# Setup Host Volumes
New-Item -ItemType "directory" -Path "C:\tmp\data" -Force

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
