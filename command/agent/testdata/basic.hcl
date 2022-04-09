# This file was used to generate basic.json from https://www.hcl2json.com/
region = "foobar"

datacenter = "dc2"

name = "my-web"

data_dir = "/tmp/nomad"

plugin_dir = "/tmp/nomad-plugins"

log_level = "ERR"

log_json = true

log_file = "/var/log/nomad.log"

bind_addr = "192.168.0.1"

enable_debug = true

ports {
  http = 1234
  rpc  = 2345
  serf = 3456
}

addresses {
  http = "127.0.0.1"
  rpc  = "127.0.0.2"
  serf = "127.0.0.3"
}

advertise {
  rpc  = "127.0.0.3"
  serf = "127.0.0.4"
}

client {
  enabled    = true
  state_dir  = "/tmp/client-state"
  alloc_dir  = "/tmp/alloc"
  servers    = ["a.b.c:80", "127.0.0.1:1234"]
  node_class = "linux-medium-64bit"

  meta {
    foo = "bar"
    baz = "zip"
  }

  server_join {
    retry_join     = ["1.1.1.1", "2.2.2.2"]
    retry_max      = 3
    retry_interval = "15s"
  }

  options {
    foo = "bar"
    baz = "zip"
  }

  chroot_env {
    "/opt/myapp/etc" = "/etc"
    "/opt/myapp/bin" = "/bin"
  }

  network_interface = "eth0"
  network_speed     = 100
  cpu_total_compute = 4444

  reserved {
    cpu            = 10
    memory         = 10
    disk           = 10
    reserved_ports = "1,100,10-12"
  }

  client_min_port  = 1000
  client_max_port  = 2000
  max_kill_timeout = "10s"

  stats {
    data_points         = 35
    collection_interval = "5s"
  }

  gc_interval              = "6s"
  gc_parallel_destroys     = 6
  gc_disk_usage_threshold  = 82
  gc_inode_usage_threshold = 91
  gc_max_allocs            = 50
  no_host_uuid             = false
  disable_remote_exec      = true

  host_volume "tmp" {
    path = "/tmp"
  }

  cni_path              = "/tmp/cni_path"
  bridge_network_name   = "custom_bridge_name"
  bridge_network_subnet = "custom_bridge_subnet"
}

server {
  enabled                       = true
  authoritative_region          = "foobar"
  bootstrap_expect              = 5
  data_dir                      = "/tmp/data"
  raft_protocol                 = 3
  num_schedulers                = 2
  enabled_schedulers            = ["test"]
  node_gc_threshold             = "12h"
  job_gc_interval               = "3m"
  job_gc_threshold              = "12h"
  eval_gc_threshold             = "12h"
  deployment_gc_threshold       = "12h"
  csi_volume_claim_gc_threshold = "12h"
  csi_plugin_gc_threshold       = "12h"
  heartbeat_grace               = "30s"
  min_heartbeat_ttl             = "33s"
  max_heartbeats_per_second     = 11.0
  failover_heartbeat_ttl        = "330s"
  retry_join                    = ["1.1.1.1", "2.2.2.2"]
  start_join                    = ["1.1.1.1", "2.2.2.2"]
  retry_max                     = 3
  retry_interval                = "15s"
  rejoin_after_leave            = true
  non_voting_server             = true
  redundancy_zone               = "foo"
  upgrade_version               = "0.8.0"
  encrypt                       = "abc"
  raft_multiplier               = 4
  enable_event_broker           = false
  event_buffer_size             = 200

  server_join {
    retry_join     = ["1.1.1.1", "2.2.2.2"]
    retry_max      = 3
    retry_interval = "15s"
  }

  default_scheduler_config {
    scheduler_algorithm = "spread"

    preemption_config {
      batch_scheduler_enabled   = true
      system_scheduler_enabled  = true
      service_scheduler_enabled = true
    }
  }

  license_path = "/tmp/nomad.hclic"
}

acl {
  enabled           = true
  token_ttl         = "60s"
  policy_ttl        = "60s"
  replication_token = "foobar"
}

audit {
  enabled = true

  sink "file" {
    type               = "file"
    delivery_guarantee = "enforced"
    format             = "json"
    path               = "/opt/nomad/audit.log"
    rotate_bytes       = 100
    rotate_duration    = "24h"
    rotate_max_files   = 10
  }

  filter "default" {
    type       = "HTTPEvent"
    endpoints  = ["/v1/metrics"]
    stages     = ["*"]
    operations = ["*"]
  }
}

telemetry {
  statsite_address           = "127.0.0.1:1234"
  statsd_address             = "127.0.0.1:2345"
  prometheus_metrics         = true
  disable_hostname           = true
  collection_interval        = "3s"
  publish_allocation_metrics = true
  publish_node_metrics       = true
}

leave_on_interrupt = true

leave_on_terminate = true

enable_syslog = true

syslog_facility = "LOCAL1"

disable_update_check = true

disable_anonymous_signature = true

http_api_response_headers {
  Access-Control-Allow-Origin = "*"
}

consul {
  server_service_name    = "nomad"
  server_http_check_name = "nomad-server-http-health-check"
  server_serf_check_name = "nomad-server-serf-health-check"
  server_rpc_check_name  = "nomad-server-rpc-health-check"
  client_service_name    = "nomad-client"
  client_http_check_name = "nomad-client-http-health-check"
  address                = "127.0.0.1:9500"
  allow_unauthenticated  = true
  token                  = "token1"
  auth                   = "username:pass"
  ssl                    = true
  verify_ssl             = true
  ca_file                = "/path/to/ca/file"
  cert_file              = "/path/to/cert/file"
  key_file               = "/path/to/key/file"
  server_auto_join       = true
  client_auto_join       = true
  auto_advertise         = true
  checks_use_advertise   = true
}

vault {
  address               = "127.0.0.1:9500"
  allow_unauthenticated = true
  task_token_ttl        = "1s"
  enabled               = false
  token                 = "12345"
  ca_file               = "/path/to/ca/file"
  ca_path               = "/path/to/ca"
  cert_file             = "/path/to/cert/file"
  key_file              = "/path/to/key/file"
  tls_server_name       = "foobar"
  tls_skip_verify       = true
  create_from_role      = "test_role"
}

tls {
  http                            = true
  rpc                             = true
  verify_server_hostname          = true
  ca_file                         = "foo"
  cert_file                       = "bar"
  key_file                        = "pipe"
  rpc_upgrade_mode                = true
  verify_https_client             = true
  tls_prefer_server_cipher_suites = true
  tls_cipher_suites               = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
  tls_min_version                 = "tls12"
}

sentinel {
  import "foo" {
    path = "foo"
    args = ["a", "b", "c"]
  }

  import "bar" {
    path = "bar"
    args = ["x", "y", "z"]
  }
}

autopilot {
  cleanup_dead_servers      = true
  disable_upgrade_migration = true
  last_contact_threshold    = "12705s"
  max_trailing_logs         = 17849
  min_quorum                = 3
  enable_redundancy_zones   = true
  server_stabilization_time = "23057s"
  enable_custom_upgrades    = true
}

plugin "docker" {
  args = ["foo", "bar"]

  config {
    foo = "bar"

    nested {
      bam = 2
    }
  }
}

plugin "exec" {
  config {
    foo = true
  }
}
