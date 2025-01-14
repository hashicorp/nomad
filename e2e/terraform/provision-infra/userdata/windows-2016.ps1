# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

<powershell>

# Bring ebs volume online with read-write access
Get-Disk | Where-Object IsOffline -Eq $True | Set-Disk -IsOffline $False
Get-Disk | Where-Object isReadOnly -Eq $True | Set-Disk -IsReadOnly $False

md "C:\Users\Administrator\.ssh\"

$myKey = "C:\Users\Administrator\.ssh\authorized_keys"
$adminKey = "C:\ProgramData\ssh\administrators_authorized_keys"

Invoke-RestMethod `
  -Uri "http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key" `
  -Outfile $myKey

cp $myKey $adminKey

icacls $adminKey /reset
icacls $adminKey /inheritance:r
icacls $adminKey /grant BUILTIN\Administrators:`(F`)
icacls $adminKey /grant SYSTEM:`(F`)

# for host volume testing
New-Item -ItemType Directory -Force -Path C:\tmp\data

</powershell>
