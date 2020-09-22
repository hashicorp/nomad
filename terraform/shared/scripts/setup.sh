#!/bin/bash

set -e
log() {
  echo "==> $(date) - ${1}"
}
log "Running ${0}"

log "Waiting for cloud-init to update /etc/apt/sources.list"
timeout 180 /bin/bash -c \
  'until stat /var/lib/cloud/instance/boot-finished 2>/dev/null; do echo waiting ...; sleep 1; done'

# Disable interactive apt prompts
export DEBIAN_FRONTEND=noninteractive
echo 'debconf debconf/frontend select Noninteractive' | sudo debconf-set-selections

# Dependencies
sudo apt-get update
sudo apt-get install -y \
	software-properties-common \
	unzip \
	tree \
	redis-tools \
	jq \
	curl \
	tmux \
	apt-transport-https \
	ca-certificates \
	lsb-release \
	sudo \
	vim \
	gnupg2 

cd /ops

CONFIGDIR=/ops/shared/config

# This function will facilitate transition to package installs; however
# post-provisioning is likely to need changing to work with packaged
# installs.

install_packages()
{
    log "Installing HashiCorp Packages"
    curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
    sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
    sudo apt-get update
    sudo apt-get install nomad consul vault terraform packer
    sudo apt-get clean all
}

install_zip()
{
    NAME="$1"
    DOWNLOAD_URL="$2"
    log "Installing ${NAME} from ${DOWNLOAD_URL}"
    curl -sS -L -o ~/$NAME.zip $DOWNLOAD_URL
    sudo unzip -d /usr/local/bin/ ~/$NAME.zip
    sudo chmod 0755 /usr/local/bin/$NAME
    sudo chown root:root /usr/local/bin/$NAME
    rm ~/$NAME.zip
}

install_release()
{
    PRODUCT="$1"
    VERSION="$2"
    DOWNLOAD_URL="https://releases.hashicorp.com/${PRODUCT}/${VERSION}/${PRODUCT}_${VERSION}_linux_amd64.zip"
    install_zip "$PRODUCT" "$DOWNLOAD_URL"
}

install_release "consul" "1.8.3"
install_release "vault" "1.5.3"
install_release "nomad" "0.12.5"
install_release "packer" "1.6.2"
install_release "terraform" "0.13.2"
install_release "consul-template" "0.25.1"
install_release "envconsul" "0.10.0"
install_release "sentinel" "0.15.6"

CONSULCONFIGDIR=/etc/consul.d
CONSULDIR=/opt/consul

VAULTCONFIGDIR=/etc/vault.d
VAULTDIR=/opt/vault

NOMADCONFIGDIR=/etc/nomad.d
NOMADDIR=/opt/nomad

CONSULTEMPLATECONFIGDIR=/etc/consul-template.d
CONSULTEMPLATEDIR=/opt/consul-template

# Disable the firewall

sudo ufw disable || echo "ufw not installed"

# Consul
## Configure
sudo mkdir -p $CONSULCONFIGDIR $CONSULDIR
sudo chmod 755 $CONSULCONFIGDIR $CONSULDIR

# Vault
## Configure
sudo mkdir -p $VAULTCONFIGDIR $VAULTDIR
sudo chmod 755 $VAULTCONFIGDIR $VAULTDIR

# Nomad
## Configure
sudo mkdir -p $NOMADCONFIGDIR $NOMADDIR
sudo chmod 755 $NOMADCONFIGDIR $NOMADDIR

# Consul Template 
## Configure
sudo mkdir -p $CONSULTEMPLATECONFIGDIR $CONSULTEMPLATEDIR
sudo chmod 755 $CONSULTEMPLATECONFIGDIR $CONSULTEMPLATEDIR

## Everything below here is installed in service to Nomad Clients.
## Should this be done in a different image?


# Docker
distro=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
sudo apt-get install -y apt-transport-https ca-certificates gnupg2 
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/${distro} $(lsb_release -cs) stable"
sudo apt-get update
sudo apt-get install -y docker-ce

# Needs testing, updating and fixing
if [[ ! -z ${INSTALL_NVIDIA_DOCKER+x} ]]; then 
  # Install official NVIDIA driver package
  sudo apt-key adv --fetch-keys http://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64/7fa2af80.pub
  sudo sh -c 'echo "deb http://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64 /" > /etc/apt/sources.list.d/cuda.list'
  sudo apt-get update && sudo apt-get install -y --no-install-recommends linux-headers-generic dkms cuda-drivers

  # Install nvidia-docker and nvidia-docker-plugin
  # from: https://github.com/NVIDIA/nvidia-docker#ubuntu-140416041804-debian-jessiestretch
  wget -P /tmp https://github.com/NVIDIA/nvidia-docker/releases/download/v1.0.1/nvidia-docker_1.0.1-1_amd64.deb
  sudo dpkg -i /tmp/nvidia-docker*.deb && rm /tmp/nvidia-docker*.deb
  curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
  distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
  curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
    sudo tee /etc/apt/sources.list.d/nvidia-docker.list

  sudo apt-get update
  sudo apt-get install -y nvidia-docker2
fi

# rkt
# Note: rkt has been ended and archived. This should likely be removed. 
# See https://github.com/rkt/rkt/issues/4024
VERSION=1.30.0
DOWNLOAD=https://github.com/rkt/rkt/releases/download/v${VERSION}/rkt-v${VERSION}.tar.gz

function install_rkt() {
	wget -q -O /tmp/rkt.tar.gz "${DOWNLOAD}"
	tar -C /tmp -xvf /tmp/rkt.tar.gz
	sudo mv /tmp/rkt-v${VERSION}/rkt /usr/local/bin
	sudo mv /tmp/rkt-v${VERSION}/*.aci /usr/local/bin
}

function configure_rkt_networking() {
	sudo mkdir -p /etc/rkt/net.d
    sudo bash -c 'cat << EOT > /etc/rkt/net.d/99-network.conf
{
  "name": "default",
  "type": "ptp",
  "ipMasq": false,
  "ipam": {
    "type": "host-local",
    "subnet": "172.16.28.0/24",
    "routes": [
      {
        "dst": "0.0.0.0/0"
      }
    ]
  }
}
EOT'
}

install_rkt
configure_rkt_networking

# Java
sudo add-apt-repository -y ppa:openjdk-r/ppa
sudo apt-get update 
sudo apt-get install -y openjdk-8-jdk
JAVA_HOME=$(readlink -f /usr/bin/java | sed "s:bin/java::")
