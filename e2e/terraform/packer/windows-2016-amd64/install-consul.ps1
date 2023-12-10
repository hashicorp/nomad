# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-Location C:\opt

Try {
    $releases = "https://releases.hashicorp.com"
    $version = "1.11.4+ent"
    $url = "${releases}/consul/${version}/consul_${version}_windows_amd64.zip"

    New-Item -ItemType Directory -Force -Path C:\opt\consul
    New-Item -ItemType Directory -Force -Path C:\etc\consul.d

    # TODO: check sha!
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
