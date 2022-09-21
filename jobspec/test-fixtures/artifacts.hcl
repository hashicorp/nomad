job "binstore-storagelocker" {
  group "binsl" {
    task "binstore" {
      driver = "docker"

      artifact {
        source      = "http://foo.com/bar"
        destination = ""

        options {
          foo = "bar"
        }
      }

      artifact {
        source = "http://foo.com/baz"
      }

      artifact {
        source      = "http://foo.com/bam"
        destination = "var/foo"
      }

      artifact {
        source = "https://example.com/file.txt"

        headers {
          User-Agent    = "nomad-[${NOMAD_JOB_ID}]-[${NOMAD_GROUP_NAME}]-[${NOMAD_TASK_NAME}]"
          X-Nomad-Alloc = "${NOMAD_ALLOC_ID}"
        }
      }
    }
  }
}
