Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-Location C:\opt

Try {
    $releases = "https://releases.hashicorp.com"
    $version = "1.7.3"
    $url = "${releases}/consul/${version}/consul_${version}_windows_amd64.zip"

    New-Item -ItemType Directory -Force -Path C:\opt\consul
    New-Item -ItemType Directory -Force -Path C:\opt\consul.d

    # TODO: check sha!
    Write-Output "Downloading Consul from: $url"
    Invoke-WebRequest -Uri $url -Outfile consul.zip
    Expand-Archive .\consul.zip .\
    mv consul.exe C:\opt\consul.exe
    C:\opt\consul.exe version
    rm consul.zip

} Catch {
    Write-Error "Failed to install Consul."
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed Consul."
