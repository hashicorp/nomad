param(
    [string]$nomad_sha,
    [string]$nomad_version,
    [string]$nomad_binary,
    [switch]$enterprise = $false,
    [switch]$nomad_acls = $false,
    [string]$config_profile,
    [string]$role,
    [string]$index,
    [string]$autojoin,
    [switch]$nostart = $false
)

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"

$usage = @"
Usage: provision.ps1 [options...]
Options (use one of the following):
 -nomad_sha SHA          full git sha to install from S3
 -nomad_version VERSION  release version number (ex. 0.12.4+ent)
 -nomad_binary FILEPATH  path to file on host

Options for configuration:
 -config_profile FILEPATH path to config profile directory
 -role ROLE               role within config profile directory
 -index INDEX             count of instance, for profiles with per-instance config
 -nostart                 do not start or restart Nomad
 -enterprise              if nomad_sha is passed, use the ENT version
--autojoin                 the AWS ConsulAutoJoin tag value

"@

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12


$install_path = "C:\opt\nomad.exe"
$platform = "windows_amd64"

Set-Location C:\opt

function Usage {
    Write-Output "${usage}"
}

function InstallFromS3 {

    Try {
        # check that we don't already have this version
        if (C:\opt\nomad.exe -version `
          | Select-String -Pattern $nomad_sha -SimpleMatch -Quiet) {
              Write-Output "${nomad_sha} already installed"
              return
          }
    } Catch {
        Write-Output "${nomad_sha} not previously installed"
    }

    Stop-Service -Name nomad -ErrorAction Ignore

    $build_folder = "builds-oss"
    if ($enterprise) {
        $build_folder = "builds-ent"
    }
    $key = "${build_folder}/nomad_${platform}_${nomad_sha}.zip"

    Write-Output "Downloading Nomad from s3: $key"
    Try {
        Remove-Item -Path ./nomad.zip -Force -ErrorAction Ignore
        Read-S3Object -BucketName nomad-team-dev-test-binaries `
          -Key $key -File ./nomad.zip -ErrorAction Stop

        Remove-Item -Path $install_path -Force -ErrorAction Stop
        Expand-Archive ./nomad.zip ./ -Force -ErrorAction Stop
        Move-Item `
          -Path .\pkg\windows_amd64\nomad.exe `
          -Destination $install_path -Force -ErrorAction Stop
        Remove-Item -Path nomad.zip -Force -ErrorAction Ignore

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d -ErrorAction Stop
        New-Item -ItemType Directory -Force -Path C:\opt\nomad -ErrorAction Stop
    } Catch {
        Write-Output "Failed to install Nomad."
        Write-Output $_
        Write-Host $_.ScriptStackTrace
        $host.SetShouldExit(-1)
        throw
    }

    Write-Output "Installed Nomad."
}

function InstallFromUploadedBinary {

    Stop-Service -Name nomad -ErrorAction Ignore

    Try {
        Remove-Item -Path $install_path -Force -ErrorAction Ignore
        Move-Item -Path $nomad_binary -Destination $install_path -Force -ErrorAction Stop

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d -ErrorAction Stop
        New-Item -ItemType Directory -Force -Path C:\opt\nomad -ErrorAction Stop
    } Catch {
        Write-Output "Failed to install Nomad."
        Write-Output $_
        $host.SetShouldExit(-1)
        throw
    }

    Write-Output "Installed Nomad."
}

function InstallFromRelease {
    Try {
        # check that we don't already have this version
        if (C:\opt\nomad.exe -version `
          | Select-String -Pattern $nomad_version -SimpleMatch -Quiet) {
              if (C:\opt\nomad.exe -version `
                | Select-String -Pattern dev -SimpleMatch -Quiet -NotMatch) {
                    Write-Output "${nomad_version} already installed"
                    return
                }
          }
    } Catch {
        Write-Output "${nomad_version} not previously installed"
    }

    Stop-Service -Name nomad -ErrorAction Ignore

    $releases = "https://releases.hashicorp.com"
    $url = "${releases}/nomad/${nomad_version}/nomad_${nomad_version}_${platform}.zip"

    Write-Output "Downloading Nomad from: $url"
    Try {
        Remove-Item -Path ./nomad.zip -Force -ErrorAction Ignore
        Invoke-WebRequest -Uri $url -Outfile nomad.zip -ErrorAction Stop

        Remove-Item -Path $install_path -Force -ErrorAction Ignore
        Expand-Archive .\nomad.zip .\ -ErrorAction Stop
        Remove-Item -Path nomad.zip -Force -ErrorAction Ignore

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d -ErrorAction Stop
        New-Item -ItemType Directory -Force -Path C:\opt\nomad -ErrorAction Stop
    } Catch {
        Write-Output "Failed to install Nomad."
        Write-Output $_
        $host.SetShouldExit(-1)
        throw
    }

    Write-Output "Installed Nomad."
}


function ConfigFiles($src, $dest) {
    Get-ChildItem -Path "$src" -Name -Attributes !Directory -ErrorAction Ignore`
      | ForEach-Object { `
          New-Item -ItemType SymbolicLink -Path "${dest}\$_" -Target "${src}\$_" }
}

function InstallConfigProfile {

    if ( Test-Path -Path 'C:\tmp\custom' -PathType Container ) {
        Remote-Item 'C:\opt\config\custom' -Force -ErrorAction Ignore
        Move-Item -Path 'C:\tmp\custom' -Destination 'C:\opt\config\custom' -Force
    }

    $cfg = "C:\opt\config\${config_profile}"

    Remove-Item "C:\opt\nomad.d\*" -Force -ErrorAction Ignore
    Remove-Item "C:\opt\consul.d\*" -Force -ErrorAction Ignore

    ConfigFiles "${cfg}\nomad" "C:\opt\nomad.d"
    ConfigFiles "${cfg}\consul" "C:\opt\consul.d"

    if ( "" -ne $role ) {
        ConfigFiles "${cfg}\nomad\${role}" "C:\opt\nomad.d"
        ConfigFiles "${cfg}\consul\${role}" "C:\opt\consul.d"
    }

    if ( "" -ne $index ) {
        ConfigFiles "${cfg}\nomad\${role}\indexed\*${index}*" "C:\opt\nomad.d"
        ConfigFiles "${cfg}\consul\${role}\indexed\*${index}*" "C:\opt\consul.d"
    }
}

function UpdateConsulAutojoin {
    (Get-Content C:\opt\consul.d\aws.json).replace("tag_key=ConsulAutoJoin tag_value=auto-join", "tag_key=ConsulAutoJoin tag_value=${autojoin}") | `
      Set-Content C:\opt\consul.d\aws.json
}

function CreateConsulService {
    New-Service `
      -Name "Consul" `
      -BinaryPathName "C:\opt\consul.exe agent -config-dir C:\opt\consul.d" `
      -StartupType "Automatic" `
      -ErrorAction Ignore
}

function CreateNomadService {
    New-NetFirewallRule `
      -DisplayName 'Nomad HTTP Inbound' `
      -Profile @('Public', 'Domain', 'Private') `
      -Direction Inbound `
      -Action Allow `
      -Protocol TCP `
      -LocalPort @('4646')

    # idempotently enable as a service
    New-Service `
      -Name "Nomad" `
      -BinaryPathName "C:\opt\nomad.exe agent -config C:\opt\nomad.d" `
      -StartupType "Automatic" `
      -ErrorAction Ignore
}

if ( "" -ne $nomad_sha ) {
    InstallFromS3
    CreateNomadService
}
if ( "" -ne $nomad_version ) {
    InstallFromRelease
    CreateNomadService
}
if ( "" -ne $nomad_binary ) {
    InstallFromUploadedBinary
    CreateNomadService
}
if ( "" -ne $config_profile) {
    InstallConfigProfile
}
if ( "" -ne $autojoin) {
    UpdateConsulAutojoin
}

if (!($nostart)) {
    CreateConsulService
    CreateNomadService
    Restart-Service "Consul"
    Restart-Service "Nomad"
}
