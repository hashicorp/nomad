job "bench" {
	datacenters = ["nyc3"]

	group "cache" {
		count = 10000

		task "hello-world" {
			driver = "docker"

			config {
				image = "hello-world"
			}

			resources {
				cpu = 100
                memory = 100
			}
		}
	}
}
