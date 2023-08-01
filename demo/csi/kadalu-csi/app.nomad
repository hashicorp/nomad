variable "cn_network" {
  default = "dc1"
}

variable "vol-id" {
  default = "csi-test"
}

job "sample-pv-check" {
  datacenters = ["${var.cn_network}"]

  group "apps" {
    volume "test" {
      type            = "csi"
      source          = "${var.vol-id}"
      access_mode     = "multi-node-multi-writer"
      attachment_mode = "file-system"
    }

    task "sample" {
      # To verify volume is mounted correctly and accessible, please run
      # 'nomad alloc exec <alloc_id> bash /kadalu/script.sh'
      # after this job is scheduled and running on a nomad client
      driver = "docker"

      config {
        image      = "kadalu/sample-pv-check-app:latest"
        force_pull = false

        entrypoint = [
          "tail",
          "-f",
          "/dev/null",
        ]
      }

      volume_mount {
        volume = "test"

        # Script in this image looks for PV mounted at '/mnt/pv'
        destination = "/mnt/pv"
      }
    }
  }
}
