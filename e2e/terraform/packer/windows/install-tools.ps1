Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# TODO(tgross): we should probably get OpenSSH on these machines
# so that folks don't have to install powershell and do winrm

# TODO (tgross: some stuff installed on Linux but not here yet
# - Possible issues: no redis-tools for windows
# - Possible non-issues: probably don't need tree, curl,tmux

Try {
    Set-PSRepository -InstallationPolicy Trusted -Name PSGallery

    Write-Output "Installing 7Zip"
    Install-Package -Force 7Zip4PowerShell

    Write-Output "Installing JQ"
    Invoke-WebRequest `
      -Uri https://github.com/stedolan/jq/releases/download/jq-1.6/jq-win64.exe `
      -Outfile jq-win64.exe

    Write-Output "Installing AWS CLI"
    Invoke-WebRequest `
      -Uri https://s3.amazonaws.com/aws-cli/AWSCLI64PY3.msi `
      -Outfile AWSCLI64PY3.msi
    Start-Process msiexec.exe -Wait -ArgumentList '/I .\AWSCLI64PY3.msi /quiet'

} Catch {
    Write-Error "Failed to install dependencies."
    $host.SetShouldExit(-1)
    throw
} Finally {
    # clean up by re-securing this package repo
    Set-PSRepository -InstallationPolicy Untrusted -Name PSGallery
}

Write-Output "Installed dependencies"
