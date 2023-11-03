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

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Write-Output "Running User Data Script"
Write-Host "(host) Running User Data Script"

Set-ExecutionPolicy Unrestricted -Scope LocalMachine -Force -ErrorAction Ignore

# Don't set this before Set-ExecutionPolicy as it throws an error
$ErrorActionPreference = "stop"

# -------------------------------------------
# WinRM

# Remove HTTP listener
Remove-Item -Path WSMan:\Localhost\listener\listener* -Recurse

$Cert = New-SelfSignedCertificate `
  -CertstoreLocation Cert:\LocalMachine\My `
  -DnsName "packer"

New-Item `
  -Path WSMan:\LocalHost\Listener `
  -Transport HTTPS `
  -Address * `
  -CertificateThumbPrint $Cert.Thumbprint `
  -Force

Write-output "Setting up WinRM"
Write-host "(host) setting up WinRM"

cmd.exe /c winrm quickconfig -q
cmd.exe /c winrm set "winrm/config" '@{MaxTimeoutms="1800000"}'
cmd.exe /c winrm set "winrm/config/winrs" '@{MaxMemoryPerShellMB="1024"}'
cmd.exe /c winrm set "winrm/config/service" '@{AllowUnencrypted="true"}'
cmd.exe /c winrm set "winrm/config/client" '@{AllowUnencrypted="true"}'
cmd.exe /c winrm set "winrm/config/service/auth" '@{Basic="true"}'
cmd.exe /c winrm set "winrm/config/client/auth" '@{Basic="true"}'
cmd.exe /c winrm set "winrm/config/service/auth" '@{CredSSP="true"}'
cmd.exe /c winrm set "winrm/config/listener?Address=*+Transport=HTTPS" "@{Port=`"5986`";Hostname=`"packer`";CertificateThumbprint=`"$($Cert.Thumbprint)`"}"
cmd.exe /c netsh advfirewall firewall set rule group="remote administration" new enable=yes
cmd.exe /c netsh firewall add portopening TCP 5986 "Port 5986"
cmd.exe /c net stop winrm
cmd.exe /c sc config winrm start= auto
cmd.exe /c net start winrm


# -------------------------------------------
# Disks and Directories

# Bring ebs volume online with read-write access
Get-Disk | Where-Object IsOffline -Eq $True | Set-Disk -IsOffline $False
Get-Disk | Where-Object isReadOnly -Eq $True | Set-Disk -IsReadOnly $False

New-Item -ItemType Directory -Force -Path C:\opt -ErrorAction Stop

# -------------------------------------------
# SSH

Try {

    # install portable SSH instead of the Windows feature because we
    # need to target 2016
    $repo = "https://github.com/PowerShell/Win32-OpenSSH"
    $version = "v8.0.0.0p1-Beta"
    $url = "${repo}/releases/download/${version}/OpenSSH-Win64.zip"

    # TODO: check sha!
    Write-Output "Downloading OpenSSH from: $url"
    Invoke-WebRequest -Uri $url -Outfile "OpenSSH-Win64.zip" -ErrorAction Stop
    Expand-Archive ".\OpenSSH-Win64.zip" "C:\Program Files" -ErrorAction Stop
    Rename-Item -Path "C:\Program Files\OpenSSH-Win64" -NewName "OpenSSH" -ErrorAction Stop

    & "C:\Program Files\OpenSSH\install-sshd.ps1"

    # Start the service
    Start-Service sshd
    Set-Service -Name sshd -StartupType 'Automatic' -ErrorAction Stop

    Start-Service ssh-agent
    Set-Service -Name ssh-agent -StartupType 'Automatic' -ErrorAction Stop

    # Enable host firewall rule if it doesn't exist
    New-NetFirewallRule -Name sshd -DisplayName 'OpenSSH Server (sshd)' `
      -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 -ErrorAction Stop

    # Note: there appears to be a regression in recent versions of
    # Terraform for file provisioning over ssh for Windows with
    # powershell as the default shell
    # See: https://github.com/hashicorp/terraform/issues/30661
    #
    # Set powershell as the OpenSSH login shell
    # New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" `
    #   -Name DefaultShell `
    #   -Value "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe" `
    #   -PropertyType String -Force -ErrorAction Stop

    Write-Output "Installed OpenSSH."

} Catch {
    Write-Output "Failed to install OpenSSH."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
}

md "C:\Users\Administrator\.ssh\"

$myKey = "C:\Users\Administrator\.ssh\authorized_keys"
$adminKey = "C:\ProgramData\ssh\administrators_authorized_keys"

Invoke-RestMethod `
  -Uri "http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key" `
  -Outfile $myKey

cp $myKey $adminKey

icacls $adminKey /reset
icacls $adminKey /inheritance:r
icacls $adminKey /grant BUILTIN\Administrators:`(F`)
icacls $adminKey /grant SYSTEM:`(F`)

</powershell>
