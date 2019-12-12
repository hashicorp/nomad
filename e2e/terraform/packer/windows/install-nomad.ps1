Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-Location C:\opt

Try {
    # we install the most recent stable/GA release; this will be replaced
    # with the current master when we run e2e tests
    $releases = "https://releases.hashicorp.com"
    $version = "0.9.6"
    $url = "${releases}/nomad/${version}/nomad_${version}_windows_amd64.zip"

    $configDir = "C:\opt\nomad.d"
    md $configDir
    md C:\opt\nomad

    # TODO: check sha!
    Write-Output "Downloading Nomad from: $url"
    Invoke-WebRequest -Uri $url -Outfile nomad.zip
    Expand-Archive .\nomad.zip .\
    mv nomad.exe C:\opt\nomad.exe
    C:\opt\nomad.exe version
    rm nomad.zip

} Catch {
    Write-Error "Failed to install Nomad."
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed Nomad."
