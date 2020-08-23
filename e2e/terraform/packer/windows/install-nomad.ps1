param(
    [string]$nomad_sha,
    [string]$nomad_version,
    [string]$nomad_binary,
    [switch]$nostart = $false
)

Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"



$usage = @"
Usage: install-nomad [options...]
Options (use one of the following):
 --nomad_sha SHA          full git sha to install from S3
 --nomad_version VERSION  release version number (ex. 0.12.3+ent)
 --nomad_binary FILEPATH  path to file on host
 --nostart                do not start or restart Nomad
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

    Try {
        $key = "builds-oss/nomad_${platform}_${nomad_sha}.zip"
		Read-S3Object -BucketName nomad-team-dev-test-binaries -Key $key -File ./nomad.zip
		Remove-Item -Path $install_path -Force -ErrorAction Ignore
		Expand-Archive ./nomad.zip ./ -Force
		Move-Item -Path .\pkg\windows_amd64\nomad.exe -Destination $install_path -Force
		Remove-Item -Path nomad.zip -Force -ErrorAction Ignore

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d
        New-Item -ItemType Directory -Force -Path C:\opt\nomad
        Write-Output "Installed Nomad."

        if (!($nostart)) {
            StartNomad
        }
    } Catch {
        Write-Error "Failed to install Nomad."
        $host.SetShouldExit(-1)
        throw
    }
}

function InstallFromUploadedBinary {
    Try {
		Remove-Item -Path $install_path -Force -ErrorAction Ignore
        Move-Item -Path $nomad_binary -Destination $install_path -Force

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d
        New-Item -ItemType Directory -Force -Path C:\opt\nomad
        Write-Output "Installed Nomad."

        if (!($nostart)) {
            StartNomad
        }
    } Catch {
        Write-Error "Failed to install Nomad."
        $host.SetShouldExit(-1)
        throw
    }
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

    Try {
        $releases = "https://releases.hashicorp.com"
        $url = "${releases}/nomad/${nomad_version}/nomad_${nomad_version}_${platform}.zip"

        Write-Output "Downloading Nomad from: $url"
        Invoke-WebRequest -Uri $url -Outfile nomad.zip
		Remove-Item -Path $install_path -Force -ErrorAction Ignore
        Expand-Archive .\nomad.zip .\
		Remove-Item nomad.zip -Force -ErrorAction Ignore

        New-Item -ItemType Directory -Force -Path C:\opt\nomad.d
        New-Item -ItemType Directory -Force -Path C:\opt\nomad
        Write-Output "Installed Nomad."

        if (!($nostart)) {
            StartNomad
        }
    } Catch {
        Write-Error "Failed to install Nomad."
        $host.SetShouldExit(-1)
        throw
    }
}

function StartNomad {
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

    Start-Service "Nomad"
}

if ( "" -ne $nomad_sha ) {
    InstallFromS3
    return
}
if ( "" -ne $nomad_version ) {
    InstallFromRelease
    return
}
if ( "" -ne $nomad_binary ) {
    InstallFromUploadedBinary
    return
}

Usage
