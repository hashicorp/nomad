
Set-Location C:\ops

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$CONSULVERSION = "1.4.4"
$CONSULDOWNLOAD = "https://releases.hashicorp.com/consul/${CONSULVERSION}/consul_${CONSULVERSION}_windows_amd64.zip"
$CONSULCONFIGDIR = "C:\opt\consul.d"
$CONSULDIR = "C:\ops\consul"

$VAULTVERSION = "1.1.1"
$VAULTDOWNLOAD = "https://releases.hashicorp.com/vault/${VAULTVERSION}/vault_${VAULTVERSION}_windows_amd64.zip"
$VAULTCONFIGDIR = "C:\opt\vault.d"
$VAULTDIR = "C:\ops\vault"

md C:\ops\bin

Write-Output "Install Consul"
md $CONSULDIR
md $CONSULCONFIGDIR
Invoke-WebRequest -Uri $CONSULDOWNLOAD -Outfile consul.zip
Expand-Archive .\consul.zip .\
mv consul.exe .\bin\consul.exe
.\bin\consul.exe version
rm consul.zip
mv .\shared\config\consul.json $CONSULCONFIGDIR

Write-Output "Install Vault"
md $VAULTDIR
md $VAULTCONFIGDIR
Invoke-WebRequest -Uri $VAULTDOWNLOAD -Outfile vault.zip
Expand-Archive .\vault.zip .\
mv vault.exe .\bin\vault.exe
.\bin\vault.exe version

docker ps
