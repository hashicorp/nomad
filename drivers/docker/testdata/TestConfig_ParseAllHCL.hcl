# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

config {
  image                  = "redis:7"
  image_pull_timeout     = "15m"
  advertise_ipv6_address = true
  args                   = ["command_arg1", "command_arg2"]
  auth {
    username       = "myusername"
    password       = "mypassword"
    email          = "myemail@example.com"
    server_address = "https://example.com"
  }

  auth_soft_fail            = true
  cap_add                   = ["CAP_SYS_NICE"]
  cap_drop                  = ["CAP_SYS_ADMIN", "CAP_SYS_TIME"]
  command                   = "/bin/bash"
  container_exists_attempts = 10
  cgroupns                  = "host"
  cpu_hard_limit            = true
  cpu_cfs_period            = 20
  devices = [
    { "host_path" = "/dev/null", "container_path" = "/tmp/container-null", cgroup_permissions = "rwm" },
    { "host_path" = "/dev/random", "container_path" = "/tmp/container-random" },
    { "host_path" = "/dev/bus/usb" },
  ]
  dns_search_domains = ["sub.example.com", "sub2.example.com"]
  dns_options        = ["debug", "attempts:10"]
  dns_servers        = ["8.8.8.8", "1.1.1.1"]
  entrypoint         = ["/bin/bash", "-c"]
  extra_hosts        = ["127.0.0.1  localhost.example.com"]
  force_pull         = true
  group_add          = ["group1", "group2"]
  healthchecks {
    disable = true
  }
  hostname     = "self.example.com"
  interactive  = true
  ipc_mode     = "host"
  ipv4_address = "10.0.2.1"
  ipv6_address = "2601:184:407f:b37c:d834:412e:1f86:7699"
  labels = {
    owner         = "hashicorp-nomad"
    key           = "val"
    "dotted.keys" = "work"
  }
  load = "/tmp/image.tar.gz"
  logging {
    driver = "json-file-driver"
    type   = "json-file"
    config {
      "max-file" = "3"
      "max-size" = "10m"
    }
  }
  mac_address       = "02:42:ac:11:00:02"
  memory_hard_limit = 512

  mount {
    type     = "bind"
    target   = "/mount-bind-target"
    source   = "/bind-source-mount"
    readonly = true
    bind_options {
      propagation = "rshared"
    }
  }

  mount {
    type     = "tmpfs"
    target   = "/mount-tmpfs-target"
    readonly = true
    tmpfs_options {
      size = 30000
      mode = 0777
    }
  }

  mounts = [
    {
      type     = "bind"
      target   = "/bind-target",
      source   = "/bind-source"
      readonly = true
      bind_options {
        propagation = "rshared"
      }
    },
    {
      type     = "tmpfs"
      target   = "/tmpfs-target",
      readonly = true
      tmpfs_options {
        size = 30000
        mode = 0777
      }
    },
    {
      type     = "volume"
      target   = "/volume-target"
      source   = "/volume-source"
      readonly = true
      volume_options {
        no_copy = true
        labels = {
          label_key = "label_value"
          "dotted.keys" = "always work"
        }
        driver_config {
          name = "nfs"
          options {
            option_key = "option_value"
          }
        }
      }
    },
  ]
  network_aliases = ["redis"]
  network_mode    = "host"
  oom_score_adj   = 1000
  pids_limit      = 2000
  pid_mode        = "host"
  ports           = ["http", "https"]
  port_map {
    http  = 80
    redis = 6379
  }
  privileged      = true
  readonly_rootfs = true
  runtime         = "runc"
  security_opt = [
    "credentialspec=file://gmsaUser.json"
  ],
  shm_size = 30000
  storage_opt {
    dm.thinpooldev           = "dev/mapper/thin-pool"
    dm.use_deferred_deletion = "true"
    dm.use_deferred_removal  = "true"

  }
  sysctl {
    net.core.somaxconn = "16384"
  }
  tty = true
  ulimit {
    nproc  = "4242"
    nofile = "2048:4096"
  }
  uts_mode    = "host"
  userns_mode = "host"
  volumes = [
    "/host-path:/container-path:rw",
  ]
  volume_driver = "host"
  work_dir      = "/tmp/workdir"
}
