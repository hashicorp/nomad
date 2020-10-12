Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-Location C:\opt

Try {
    $releases = "https://releases.hashicorp.com"
    $version = "1.2.3"
    $url = "${releases}/vault/${version}/vault_${version}_windows_amd64.zip"

    New-Item -ItemType Directory -Force -Path C:\opt\vault
    New-Item -ItemType Directory -Force -Path C:\opt\vault.d

    # TODO: check sha!
    Write-Output "Downloading Vault from: $url"
    Invoke-WebRequest -Uri $url -Outfile vault.zip
    Expand-Archive .\vault.zip .\
    mv vault.exe C:\opt\vault.exe
    C:\opt\vault.exe version
    rm vault.zip

} Catch {
    Write-Error "Failed to install Vault."
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed Vault."
