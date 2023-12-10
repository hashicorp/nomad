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
    Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force -ErrorAction Stop
} Catch {
    Write-Output "Failed to install NuGet package manager."
    Write-Output $_
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed NuGet."
