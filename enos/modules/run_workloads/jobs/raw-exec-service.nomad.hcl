job "service-raw" {

  group "service-raw" {
    count = 1
    task "raw" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "./local/runme.sh"]
      }

      template {
        data        = <<EOH
 #!/bin/bash

sigkill_handler() {
    echo "Received SIGKILL signal. Exiting..."
    exit 0
}

echo "Sleeping until SIGKILL signal is received..."
while true; do
    sleep 300  
done
EOH
        destination = "local/runme.sh"
        perms       = "755"
      }
    }
  }
}
