job "nomad_provider_service_many_cat" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "show" {

    count = 1

    task "cat" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["15000"]
      }
      template {
        destination = "local/redis.txt"
        data = <<EOH
{{$allocID := env "NOMAD_ALLOC_ID" -}}
{{range nomadService 2 $allocID "redis"}}
  {{.Address}} {{.Port}} | {{.Tags}} @ {{.Datacenter}}
{{- end}}
EOH
      }
      resources {
        cpu = 10
        memory = 10
      }
    }
  }
}
