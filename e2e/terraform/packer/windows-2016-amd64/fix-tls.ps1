# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This script hardens TLS configuration by disabling weak and broken protocols
# and enabling useful protocols like TLS 1.1 and 1.2.

$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}

$weakProtocols = @(
	'Multi-Protocol Unified Hello',
	'PCT 1.0',
	'SSL 2.0',
	'SSL 3.0'
)

$strongProtocols = @(
	'TLS 1.0',
	'TLS 1.1',
	'TLS 1.2'
)

$weakCiphers = @(
	'DES 56/56',
	'NULL',
	'RC2 128/128',
	'RC2 40/128',
	'RC2 56/128',
	'RC4 40/128',
	'RC4 56/128',
	'RC4 64/128',
	'RC4 128/128'
)

$strongCiphers = @(
	'AES 128/128',
	'AES 256/256',
	'Triple DES 168/168'
)

$weakHashes = @(
	'MD5',
	'SHA'
)

$strongHashes = @(
	'SHA 256',
	'SHA 384',
	'SHA 512'
)

$strongKeyExchanges = @(
	'Diffie-Hellman',
	'ECDH',
	'PKCS'
)

$cipherOrder = @(
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384_P521',
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384_P384',
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384_P256',
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA_P521',
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA_P384',
  'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA_P256',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256_P521',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256_P384',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256_P256',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA_P521',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA_P384',
  'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA_P256',
  'TLS_RSA_WITH_AES_256_GCM_SHA384',
  'TLS_RSA_WITH_AES_128_GCM_SHA256',
  'TLS_RSA_WITH_AES_256_CBC_SHA256',
  'TLS_RSA_WITH_AES_256_CBC_SHA',
  'TLS_RSA_WITH_AES_128_CBC_SHA256',
  'TLS_RSA_WITH_AES_128_CBC_SHA',
  'TLS_RSA_WITH_3DES_EDE_CBC_SHA'
)

# Reset the protocols key
New-Item 'HKLM:SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols' -Force | Out-Null

# Disable weak protocols
Foreach ($protocol in $weakProtocols) {
  New-Item HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -Force | Out-Null
  New-Item HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -name Enabled -value 0 -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -name DisabledByDefault -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -name Enabled -value 0 -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -name DisabledByDefault -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
}

# Enable strong protocols
Foreach ($protocol in $strongProtocols) {
  New-Item HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -Force | Out-Null
  New-Item HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -name 'Enabled' -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Server -name 'DisabledByDefault' -value 0 -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -name 'Enabled' -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  New-ItemProperty -path HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\$protocol\Client -name 'DisabledByDefault' -value 0 -PropertyType 'DWord' -Force | Out-Null
}

# Reset the ciphers key
New-Item 'HKLM:SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers' -Force | Out-Null

# Disable Weak Ciphers
Foreach ($cipher in $weakCiphers) {
  $key = (get-item HKLM:\).OpenSubKey("SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers", $true).CreateSubKey($cipher)
  $key.SetValue('Enabled', 0, 'DWord')
  $key.Close()
}

# Enable Strong Ciphers
Foreach ($cipher in $strongCiphers) {
  $key = (get-item HKLM:\).OpenSubKey("SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers", $true).CreateSubKey($cipher)
  New-ItemProperty -path "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\$cipher" -name 'Enabled' -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  $key.Close()
}

# Reset the hashes key
New-Item 'HKLM:SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Hashes' -Force | Out-Null

# Disable weak hashes
Foreach ($hash in $weakHashes) {
  $key = (get-item HKLM:\).OpenSubKey("SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Hashes", $true).CreateSubKey($hash)
  New-ItemProperty -path "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Hashes\$hash" -name 'Enabled' -value '0' -PropertyType 'DWord' -Force | Out-Null
  $key.Close()
}

# Enable Hashes
Foreach ($hash in $strongHashes) {
  $key = (get-item HKLM:\).OpenSubKey("SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Hashes", $true).CreateSubKey($hash)
  New-ItemProperty -path "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Hashes\$hash" -name 'Enabled' -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  $key.Close()
}

# Reset the KeyExchangeAlgorithms key
New-Item 'HKLM:SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\KeyExchangeAlgorithms' -Force | Out-Null

# Enable KeyExchangeAlgorithms
Foreach ($keyExchange in $strongKeyExchanges) {
  $key = (get-item HKLM:\).OpenSubKey("SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\KeyExchangeAlgorithms", $true).CreateSubKey($keyExchange)
  New-ItemProperty -path "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\KeyExchangeAlgorithms\$keyExchange" -name 'Enabled' -value '0xffffffff' -PropertyType 'DWord' -Force | Out-Null
  $key.Close()
}

# Set cipher order
$cipherOrderString = [string]::join(',', $cipherOrder)
New-ItemProperty -path 'HKLM:\SOFTWARE\Policies\Microsoft\Cryptography\Configuration\SSL\00010002' -name 'Functions' -value $cipherOrderString -PropertyType 'String' -Force | Out-Null

Write-Output "TLS hardened."
