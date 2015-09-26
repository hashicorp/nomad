job "bench" {
	datacenters = ["nyc3"]

	group "cache" {
		count = 10000

		task "redis" {
			driver = "docker"

			config {
				image = "redis"
			}

			resources {
				cpu = 100
                memory = 100
			}
		}
	}
}
