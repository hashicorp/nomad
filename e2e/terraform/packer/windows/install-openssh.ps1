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

    # install portable SSH instead of the Windows feature because we
    # need to target 2016
    $repo = "https://github.com/PowerShell/Win32-OpenSSH"
    $version = "v8.0.0.0p1-Beta"
    $url = "${repo}/releases/download/${version}/OpenSSH-Win64.zip"

    # TODO: check sha!
    Write-Output "Downloading OpenSSH from: $url"
    Invoke-WebRequest -Uri $url -Outfile "OpenSSH-Win64.zip"
    Expand-Archive ".\OpenSSH-Win64.zip" "C:\Program Files"
    Rename-Item -Path "C:\Program Files\OpenSSH-Win64" -NewName "OpenSSH"

    & "C:\Program Files\OpenSSH\install-sshd.ps1"

    # Start the service
    Start-Service sshd
    Set-Service -Name sshd -StartupType 'Automatic'

    Start-Service ssh-agent
    Set-Service -Name ssh-agent -StartupType 'Automatic'

    # Enable host firewall rule if it doesn't exist
    New-NetFirewallRule -Name sshd -DisplayName 'OpenSSH Server (sshd)' `
      -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22

    # Set powershell as the OpenSSH login shell
    New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" `
      -Name DefaultShell `
      -Value "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe" `
      -PropertyType String -Force


} Catch {
    Write-Error "Failed to install OpenSSH."
    $host.SetShouldExit(-1)
    throw
}

Write-Output "Installed OpenSSH."
