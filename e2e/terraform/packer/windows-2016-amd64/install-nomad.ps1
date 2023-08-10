# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-Location C:\opt

Try {
    $releases = "https://releases.hashicorp.com"
    $version = "1.2.6"
    $url = "${releases}/nomad/${version}/nomad_${version}_windows_amd64.zip"

    New-Item -ItemType Directory -Force -Path C:\opt\nomad
    New-Item -ItemType Directory -Force -Path C:\etc\nomad.d

    # TODO: check sha!
    Write-Output "Downloading Nomad from: $url"
    Invoke-WebRequest -Uri $url -Outfile nomad.zip -ErrorAction Stop
    Expand-Archive .\nomad.zip .\ -ErrorAction Stop
    Move-Item nomad.exe C:\opt\nomad.exe -Force -ErrorAction Stop
    C:\opt\nomad.exe version
    rm nomad.zip

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
