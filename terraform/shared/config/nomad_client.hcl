data_dir = "/opt/nomad/data"
bind_addr = "IP_ADDRESS"
name = "nomad@IP_ADDRESS"

# Enable the client
client {
  enabled = true
  chroot_env {
    "/bin"              = "/bin"
    "/etc"              = "/etc"
    "/home"             = "/home"
    "/lib"              = "/lib"
    "/lib32"            = "/lib32"
    "/lib64"            = "/lib64"
    "/run/resolvconf"   = "/run/resolvconf"
    "/sbin"             = "/sbin"
    "/usr"              = "/usr"
  } 
}

consul {
  address = "127.0.0.1:8500"
}

vault {
  enabled = true
  address = "http://SERVER_IP_ADDRESS:8200"
}
