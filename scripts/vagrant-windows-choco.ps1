# Install choco
Set-ExecutionPolicy Bypass; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

# Install Docker
choco install -y docker-for-windows

# Install Git
choco install -y golang

# Install Consul
choco install -y consul

# Install Vault
choco install -y vault

# Install make
choco install -y make

# Install Git
choco install -y git
