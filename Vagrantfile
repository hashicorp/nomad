# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

$script = <<SCRIPT
# Install Prereq Packages
sudo apt-get update
sudo apt-get install -y build-essential curl git-core mercurial bzr libpcre3-dev pkg-config zip default-jre qemu libc6-dev-i386 silversearcher-ag jq htop vim unzip

# Setup go, for development of Nomad
SRCROOT="/opt/go"
SRCPATH="/opt/gopath"

# Get the ARCH
ARCH=`uname -m | sed 's|i686|386|' | sed 's|x86_64|amd64|'`

# Install Go
cd /tmp
wget -q https://storage.googleapis.com/golang/go1.6.linux-${ARCH}.tar.gz
tar -xf go1.6.linux-${ARCH}.tar.gz
sudo mv go $SRCROOT
sudo chmod 775 $SRCROOT
sudo chown vagrant:vagrant $SRCROOT

# Setup the GOPATH; even though the shared folder spec gives the working
# directory the right user/group, we need to set it properly on the
# parent path to allow subsequent "go get" commands to work.
sudo mkdir -p $SRCPATH
sudo chown -R vagrant:vagrant $SRCPATH 2>/dev/null || true
# ^^ silencing errors here because we expect this to fail for the shared folder

cat <<EOF >/tmp/gopath.sh
export GOPATH="$SRCPATH"
export GOROOT="$SRCROOT"
export PATH="$SRCROOT/bin:$SRCPATH/bin:\$PATH"
EOF
sudo mv /tmp/gopath.sh /etc/profile.d/gopath.sh
sudo chmod 0755 /etc/profile.d/gopath.sh
source /etc/profile.d/gopath.sh

echo Fetching Consul...
cd /tmp/
wget https://releases.hashicorp.com/consul/0.6.4/consul_0.6.4_linux_amd64.zip -O consul.zip
echo Installing Consul...
unzip consul.zip
sudo chmod +x consul
sudo mv consul /usr/bin/consul

# Install Docker
echo deb https://apt.dockerproject.org/repo ubuntu-`lsb_release -c | awk '{print $2}'` main | sudo tee /etc/apt/sources.list.d/docker.list
sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
sudo apt-get update
sudo apt-get install -y docker-engine

# Restart docker to make sure we get the latest version of the daemon if there is an upgrade
sudo service docker restart

# Make sure we can actually use docker as the vagrant user
sudo usermod -aG docker vagrant

# Setup Nomad for development
cd /opt/gopath/src/github.com/hashicorp/nomad && make bootstrap

# Install rkt
bash scripts/install_rkt.sh

# CD into the nomad working directory when we login to the VM
grep "cd /opt/gopath/src/github.com/hashicorp/nomad" ~/.profile || echo "cd /opt/gopath/src/github.com/hashicorp/nomad" >> ~/.profile
SCRIPT

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = "cbednarski/ubuntu-1404"
  config.vm.hostname = "nomad"

  config.vm.provision "shell", inline: $script, privileged: false
  config.vm.synced_folder '.', '/opt/gopath/src/github.com/hashicorp/nomad'

  # We're going to compile go and run a concurrent system, so give ourselves
  # some extra resources. Nomad will have trouble working correctly with <2 CPUs
  # so we should use at least that many.
  cpus = 2
  memory = 2048

  config.vm.provider "parallels" do |p, o|
    o.vm.box = "parallels/ubuntu-14.04"
    p.memory = memory
    p.cpus = cpus
  end

  config.vm.provider "virtualbox" do |v|
    v.memory = memory
    v.cpus = cpus
  end

  ["vmware_fusion", "vmware_workstation"].each do |p|
    config.vm.provider p do |v|
      v.memory = memory
      v.cpus = cpus
    end
  end
end
