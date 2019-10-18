# Windows Packer Build

Explanations of boilerplate

# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Set-PSRepository -InstallationPolicy Trusted -Name PSGallery
