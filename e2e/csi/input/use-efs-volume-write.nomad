# a job that mounts an EFS volume and writes its job ID as a file
job "use-efs-volume" {
  datacenters = ["dc1"]

  group "group" {
    volume "test" {
      type   = "csi"
      source = "efs-vol0"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "bash"
        args    = ["-c", "touch /test/${NOMAD_JOB_NAME}; sleep 3600"]
      }

      volume_mount {
        volume      = "test"
        destination = "/test"
        read_only   = false
      }

      resources {
        cpu    = 500
        memory = 128
      }
    }
  }
}
