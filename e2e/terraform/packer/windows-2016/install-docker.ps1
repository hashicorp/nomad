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
    Install-WindowsFeature -Name Containers

    Write-Output "Creating user for Docker."
    net localgroup docker /add
    net localgroup docker $env:USERNAME /add

    Write-Output "Installing Docker."
    Set-PSRepository -InstallationPolicy Trusted -Name PSGallery
    Install-Module -Name DockerMsftProvider -Repository PSGallery -Force
    Install-Package -Name docker -ProviderName DockerMsftProvider -Force

} Catch {
    Write-Error "Failed to install Docker."
    $host.SetShouldExit(-1)
    throw
} Finally {
    # clean up by re-securing this package repo
    Set-PSRepository -InstallationPolicy Untrusted -Name PSGallery
}

Write-Output "Installed Docker."
