job "foo" {
    task "bar" {
        driver = "docker"
        config {
            port_map {
                db = 1234
            }
        }
    }
}
