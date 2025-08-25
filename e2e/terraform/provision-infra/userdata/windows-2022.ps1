# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

<powershell>

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

# -------------------------------------------
# Disks and Directories

# Bring ebs volume online with read-write access
Get-Disk | Where-Object IsOffline -Eq $True | Set-Disk -IsOffline $False
Get-Disk | Where-Object isReadOnly -Eq $True | Set-Disk -IsReadOnly $False

New-Item -ItemType Directory -Force -Path C:\opt\nomad
New-Item -ItemType Directory -Force -Path C:\etc\nomad.d
New-Item -ItemType Directory -Force -Path C:\tmp
New-Item -ItemType Directory -Force -Path C:\opt\consul
New-Item -ItemType Directory -Force -Path C:\etc\consul.d

# -------------------------------------------
# Install Consul Agent

Set-Location C:\opt

Try {
    $releases = "https://releases.hashicorp.com"
    $version = "1.21.1+ent"
    $url = "${releases}/consul/${version}/consul_${version}_windows_amd64.zip"

    Write-Output "Downloading Consul from: $url"
    Invoke-WebRequest -Uri $url -Outfile consul.zip -ErrorAction Stop
    Expand-Archive .\consul.zip .\ -ErrorAction Stop
    Move-Item consul.exe C:\opt\consul.exe -Force -ErrorAction Stop
    C:\opt\consul.exe version
    rm consul.zip

    New-Service `
      -Name "Consul" `
      -BinaryPathName "C:\opt\consul.exe agent -config-dir C:\etc\consul.d" `
      -StartupType "Automatic" `
      -ErrorAction Ignore

} Catch {
    Write-Output "Failed to install Consul."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed Consul."

# -------------------------------------------
# Install service and firewall rules for Nomad
# Note the service can't run until we upload Nomad too

Try {
    New-NetFirewallRule `
      -DisplayName 'Nomad HTTP Inbound' `
      -Profile @('Public', 'Domain', 'Private') `
      -Direction Inbound `
      -Action Allow `
      -Protocol TCP `
      -LocalPort @('4646')

    New-Service `
      -Name "Nomad" `
      -BinaryPathName "C:\opt\nomad.exe agent -config C:\etc\nomad.d" `
      -StartupType "Automatic" `
      -ErrorAction Ignore
} Catch {
    Write-Output "Failed to install Nomad."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed Nomad."

# --------------------------------------------
# Install firewall rules required to allow tests

Try {
    New-NetFirewallRule `
      -DisplayName 'Metrics Inbound' `
      -Profile @('Public', 'Domain', 'Private') `
      -Direction Inbound `
      -Action Allow `
      -Protocol TCP `
      -LocalPort @('6120')
} Catch {
    Write-Output "Failed to install firewall rules."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
}

# -------------------------------------------
# Install and configure ssh

# Note: we don't set powershell as the default ssh shell because of
# https://github.com/hashicorp/terraform/issues/30661

# Note: this is after we install services and binaries so that we can block on
# ssh availability and not race with the provisioning steps in Terraform

Write-Host 'Installing and starting sshd'
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Set-Service -Name sshd -StartupType Automatic
Start-Service sshd

Write-Host 'Installing and starting ssh-agent'
Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0
Set-Service -Name ssh-agent -StartupType Automatic
Start-Service ssh-agent

# From https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell&pivots=windows-server-2022
# Confirm the Firewall rule is configured. It should be created automatically by
# setup. Run the following to verify
if (!(Get-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -ErrorAction SilentlyContinue | Select-Object Name, Enabled)) {
    Write-Output "Firewall Rule 'OpenSSH-Server-In-TCP' does not exist, creating it..."
    New-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -DisplayName 'OpenSSH Server (sshd)' -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22
} else {
    Write-Output "Firewall rule 'OpenSSH-Server-In-TCP' has been created and exists."
}

md "C:\Users\Administrator\.ssh\"

$myKey = "C:\Users\Administrator\.ssh\authorized_keys"
$adminKey = "C:\ProgramData\ssh\administrators_authorized_keys"

# Manually save the private key from instance metadata
$ImdsToken = Invoke-RestMethod -Uri 'http://169.254.169.254/latest/api/token' -Method 'PUT' -Headers @{'X-aws-ec2-metadata-token-ttl-seconds' = 5400} -UseBasicParsing

$ImdsHeaders = @{'X-aws-ec2-metadata-token' = $ImdsToken}
Invoke-RestMethod -Uri 'http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key' -Headers $ImdsHeaders -UseBasicParsing -Outfile $myKey

cp $myKey $adminKey

icacls $adminKey /reset
icacls $adminKey /inheritance:r
icacls $adminKey /grant BUILTIN\Administrators:`(F`)
icacls $adminKey /grant SYSTEM:`(F`)

# Ensure the SSH agent pulls in the new key.
Restart-Service -Name ssh-agent

# -------------------------------------------
# Disable automatic updates so we don't get restarts in the middle of tests

$service = Get-WmiObject Win32_Service -Filter 'Name="wuauserv"'

if (!$service) {
  Write-Error "Failed to retrieve the wauserv service"
  exit 1
}

if ($service.StartMode -ne "Disabled") {
  $result = $service.ChangeStartMode("Disabled").ReturnValue
  if($result) {
    Write-Error "Failed to disable the 'wuauserv' service. The return value was $result."
    exit 1
  }
}

if ($service.State -eq "Running") {
  $result = $service.StopService().ReturnValue
  if ($result) {
    Write-Error "Failed to stop the 'wuauserv' service. The return value was $result."
    exit 1
  }
}

Write-Output "Automatic Windows Updates disabled."

</powershell>
