client {
	reserved {
		network {
			device = "eth0"
			ip = "127.0.0.1"
			mbits = 100
			reserved_ports = "1,100,10-12"
		}
		network {
			device = "eth1"
			ip = "128.0.0.1"
			mbits = 105
			reserved_ports = "1-1,2-4,100,102,10-12"
		}
	}
}
