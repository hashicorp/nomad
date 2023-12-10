# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

$service = Get-WmiObject Win32_Service -Filter 'Name="wuauserv"'

if (!$service) {
  Write-Error "Failed to retrieve the wauserv service"
  exit 1
}

if ($service.StartMode -ne "Disabled") {
  $result = $service.ChangeStartMode("Disabled").ReturnValue
  if($result) {
    Write-Error "Failed to disable the 'wuauserv' service. The return value was $result."
    exit 1
  }
}

if ($service.State -eq "Running") {
  $result = $service.StopService().ReturnValue
  if ($result) {
    Write-Error "Failed to stop the 'wuauserv' service. The return value was $result."
    exit 1
  }
}

Write-Output "Automatic Windows Updates disabled."
