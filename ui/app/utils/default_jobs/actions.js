/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `job "redis-actions" {
  datacenters = ["*"]

  group "cache" {
    count = 1
    network {
      port "db" {}
    }

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
        ports = ["db"]
        command = "/bin/sh"
        args = ["-c", "redis-server --port \${NOMAD_PORT_db} & /local/db_log.sh"]
      }

      resources {
        cpu = 500
        memory = 256
      }

      service {
        name = "redis-service"
        port = "db"
        provider = "nomad"

        check {
          name     = "alive"
          type     = "tcp"
          port     = "db"
          interval = "10s"
          timeout  = "2s"
        }
      }

      template {
        data = <<EOF
          #!/bin/sh
          while true; do
            echo "$(date): Current DB Size: $(redis-cli -p \${NOMAD_PORT_db} DBSIZE)"
            sleep 3
          done
EOF
        destination = "local/db_log.sh"
        perms = "0755"
      }

      # Adds a random key/value to the Redis database
      action "add-random-key" {
        command = "/bin/sh"
        args    = ["-c", "key=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13); value=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13); redis-cli -p \${NOMAD_PORT_db} SET $key $value; echo Key $key added with value $value"]
      }

      # Adds a random key/value with a "temp_" prefix to the Redis database
      action "add-random-temporary-key" {
        command = "/bin/sh"
        args    = ["-c", "key=temp_$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13); value=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13); redis-cli -p \${NOMAD_PORT_db} SET $key $value; echo Key $key added with value $value"]
      }

      # Lists all keys currently stored in the Redis database.
      action "list-keys" {
        command = "/bin/sh"
        args    = ["-c", "redis-cli -p \${NOMAD_PORT_db} KEYS '*'"]
      }

      # Retrieves various stats about the Redis server
      action "get-redis-stats" {
        command = "/bin/sh"
        args    = ["-c", "redis-cli -p \${NOMAD_PORT_db} INFO"]
      }

      # Performs a latency check of the Redis server.
      # This action is a non-terminating action, meaning it will run indefinitely until it is stopped.
      # Pass an escape sequence (Ctrl-C) to stop the action.
      action "health-check" {
        command = "/bin/sh"
        args    = ["-c", "redis-cli -p \${NOMAD_PORT_db} --latency"]
      }

      # Deletes all keys with a 'temp_' prefix
      action "flush-temp-keys" {
        command = "/bin/sh"
        args    = ["-c", <<EOF
          keys_to_delete=$(redis-cli -p \${NOMAD_PORT_db} --scan --pattern 'temp_*')
          if [ -n "$keys_to_delete" ]; then
            # Count the number of keys to delete
            deleted_count=$(echo "$keys_to_delete" | wc -l)
            # Execute the delete command
            echo "$keys_to_delete" | xargs redis-cli -p \${NOMAD_PORT_db} DEL
          else
            deleted_count=0
          fi
          remaining_keys=$(redis-cli -p \${NOMAD_PORT_db} DBSIZE)
          echo "$deleted_count temporary keys removed; $remaining_keys keys remaining in database"
EOF
        ]
      }


      # Toggles saving to disk (RDB persistence). When enabled, allocation logs will indicate a save every 60 seconds.
      action "toggle-save-to-disk" {
        command = "/bin/sh"
        args    = ["-c", <<EOF
          current_config=$(redis-cli -p \${NOMAD_PORT_db} CONFIG GET save | awk 'NR==2');
          if [ -z "$current_config" ]; then
            # Enable saving to disk (example: save after 60 seconds if at least 1 key changed)
            redis-cli -p \${NOMAD_PORT_db} CONFIG SET save "60 1";
            echo "Saving to disk enabled: 60 seconds interval if at least 1 key changed";
          else
            # Disable saving to disk
            redis-cli -p \${NOMAD_PORT_db} CONFIG SET save "";
            echo "Saving to disk disabled";
          fi;
EOF
        ]
      }


    }
  }

}
`;
