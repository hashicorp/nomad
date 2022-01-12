# See command/agent/config_test.go#TestConifg_Validate_Bad

client {
  cpu_total_compute = -10
  memory_total_mb   = -11

  max_dynamic_port = 999998
  min_dynamic_port = 999999

  gc_interval          = "-5m"
  gc_parallel_destroys = -12

  reserved {
    cpu            = 1
    memory         = 2
    disk           = -3
    reserved_ports = "1,10-99999999"
  }

  host_network "bad" {
    reserved_ports = "-10"
  }
}
