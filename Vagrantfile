# -*- mode: ruby -*-
# vi: set ft=ruby :
#

LINUX_BASE_BOX = "bento/ubuntu-16.04"
FREEBSD_BASE_BOX = "jen20/FreeBSD-11.1-RELEASE"

Vagrant.configure(2) do |config|
	# Compilation and development boxes
	config.vm.define "linux", autostart: true, primary: true do |vmCfg|
		vmCfg.vm.box = LINUX_BASE_BOX
		vmCfg.vm.hostname = "linux"
		vmCfg = configureProviders vmCfg,
			cpus: suggestedCPUCores()

		vmCfg = configureLinuxProvisioners(vmCfg)

		vmCfg.vm.synced_folder '.',
			'/opt/gopath/src/github.com/hashicorp/nomad'

		vmCfg.vm.provision "shell",
			privileged: false,
			path: './scripts/vagrant-linux-unpriv-bootstrap.sh'

        # Expose the nomad api and ui to the host
        vmCfg.vm.network "forwarded_port", guest: 4646, host: 4646, auto_correct: true

        # Expose Ember ports to the host (one for the site, one for livereload)
        vmCfg.vm.network :forwarded_port, guest: 4201, host: 4201, auto_correct: true
        vmCfg.vm.network :forwarded_port, guest: 49153, host: 49153, auto_correct: true
	end

	config.vm.define "freebsd", autostart: false, primary: false do |vmCfg|
		vmCfg.vm.box = FREEBSD_BASE_BOX
		vmCfg.vm.hostname = "freebsd"
		vmCfg = configureProviders vmCfg,
			cpus: suggestedCPUCores()

		vmCfg.vm.synced_folder '.',
			'/opt/gopath/src/github.com/hashicorp/nomad',
			type: "nfs",
			bsd__nfs_options: ['noatime']

		vmCfg.vm.provision "shell",
			privileged: true,
			path: './scripts/vagrant-freebsd-priv-config.sh'

		vmCfg.vm.provision "shell",
			privileged: false,
			path: './scripts/vagrant-freebsd-unpriv-bootstrap.sh'
	end

	# Test Cluster (Linux)
	1.upto(3) do |n|
		serverName = "nomad-server%02d" % [n]
		clientName = "nomad-client%02d" % [n]
		serverIP = "10.199.0.%d" % [10 + n]
		clientIP = "10.199.0.%d" % [20 + n]

		config.vm.define serverName, autostart: false, primary: false do |vmCfg|
			vmCfg.vm.box = LINUX_BASE_BOX
			vmCfg.vm.hostname = serverName
			vmCfg = configureProviders(vmCfg)
			vmCfg = configureLinuxProvisioners(vmCfg)

			vmCfg.vm.provider "virtualbox" do |_|
				vmCfg.vm.network :private_network, ip: serverIP
			end

			vmCfg.vm.synced_folder '.',
				'/opt/gopath/src/github.com/hashicorp/nomad'

			vmCfg.vm.provision "shell",
				privileged: true,
				path: './scripts/vagrant-linux-priv-zeroconf.sh'
		end

		config.vm.define clientName, autostart: false, primary: false do |vmCfg|
			vmCfg.vm.box = LINUX_BASE_BOX
			vmCfg.vm.hostname = clientName
			vmCfg = configureProviders(vmCfg)
			vmCfg = configureLinuxProvisioners(vmCfg)

			vmCfg.vm.provider "virtualbox" do |_|
				vmCfg.vm.network :private_network, ip: clientIP
			end

			vmCfg.vm.synced_folder '.',
				'/opt/gopath/src/github.com/hashicorp/nomad'

			vmCfg.vm.provision "shell",
				privileged: true,
				path: './scripts/vagrant-linux-priv-zeroconf.sh'
		end
	end
end

def configureLinuxProvisioners(vmCfg)
	vmCfg.vm.provision "shell",
		privileged: true,
		inline: 'rm -f /home/vagrant/linux.iso'

	vmCfg.vm.provision "shell",
		privileged: true,
		path: './scripts/vagrant-linux-priv-go.sh'

	vmCfg.vm.provision "shell",
		privileged: true,
		path: './scripts/vagrant-linux-priv-config.sh'

	vmCfg.vm.provision "shell",
		privileged: true,
		path: './scripts/vagrant-linux-priv-consul.sh'

	vmCfg.vm.provision "shell",
		privileged: true,
		path: './scripts/vagrant-linux-priv-vault.sh'

	vmCfg.vm.provision "shell",
		privileged: true,
		path: './scripts/vagrant-linux-priv-rkt.sh'

	vmCfg.vm.provision "shell",
		privileged: false,
		path: './scripts/vagrant-linux-priv-ui.sh'

	return vmCfg
end

def configureProviders(vmCfg, cpus: "2", memory: "2048")
	vmCfg.vm.provider "virtualbox" do |v|
		v.customize ["modifyvm", :id, "--cableconnected1", "on"]
		v.memory = memory
		v.cpus = cpus
	end

	["vmware_fusion", "vmware_workstation"].each do |p|
		vmCfg.vm.provider p do |v|
			v.enable_vmrun_ip_lookup = false
			v.vmx["memsize"] = memory
			v.vmx["numvcpus"] = cpus
		end
	end

	vmCfg.vm.provider "virtualbox" do |v|
		v.customize ["modifyvm", :id, "--cableconnected1", "on"]
		v.memory = memory
		v.cpus = cpus
	end

	return vmCfg
end

def suggestedCPUCores()
	case RbConfig::CONFIG['host_os']
	when /darwin/
		Integer(`sysctl -n hw.ncpu`) / 2
	when /linux/
		Integer(`grep -c ^processor /proc/cpuinfo`) / 2
	else
		2
	end
end
