job "binstore-storagelocker" {
    group "binsl" {
        local_disk {
            disk = 500
        }
        local_disk {
            disk = 100
        }
        count = 5
        task "binstore" {
            driver = "docker"

            resources {
                cpu = 500
                memory = 128
            }

            resources {
                cpu = 500
                memory = 128
            }
        }
    }
}
