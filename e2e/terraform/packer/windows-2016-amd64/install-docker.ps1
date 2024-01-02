# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Try {
    Write-Output "Installing containers feature."
    Install-WindowsFeature -Name Containers -ErrorAction Stop

    Write-Output "Creating user for Docker."
    net localgroup docker /add
    net localgroup docker $env:USERNAME /add

    Write-Output "Installing Docker."

    # Getting an error at this step? Check for their "status page" at:
    # https://github.com/PowerShell/PowerShellGallery/blob/master/psgallery_status.md
    Set-PSRepository -InstallationPolicy Trusted -Name PSGallery -ErrorAction Stop

    Install-Module -Name DockerMsftProvider -Repository PSGallery -Force -ErrorAction Stop
    Install-Package -Name docker -ProviderName DockerMsftProvider -Force -ErrorAction Stop

} Catch {
    Write-Output "Failed to install Docker."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
} Finally {
    # clean up by re-securing this package repo
    Set-PSRepository -InstallationPolicy Untrusted -Name PSGallery
}

Write-Output "Installed Docker."
