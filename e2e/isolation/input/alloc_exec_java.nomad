job "java_sleep" {
  datacenters = ["dc1"]
  type        = "batch"

  group "java" {

    task "build" {
      lifecycle {
        hook    = "prestart"
        sidecar = false
      }

      driver = "exec"
      config {
        command = "javac"
        args    = ["-d", "${NOMAD_ALLOC_DIR}", "local/Sleep.java"]
      }

      template {
        destination = "local/Sleep.java"
        data        = <<EOH
public class Sleep {
    public static void main(String... s) throws Exception {
        Thread.sleep(30000);
    }
}
EOH
      }
    }

    task "sleep" {
      driver = "java"
      config {
        class_path = "${NOMAD_ALLOC_DIR}"
        class      = "Sleep"
      }
    }
  }
}