job "foo" {
    datacenters = ["dc1"]
    group "bar" {
        count = 3
        service {
            name = "foo"
            port = "foo"
            meta {
                foo = "bar"
            }
            connect {
                sidecar_service {
                    proxy {
                        config {
                            foo = "bar"
                            number = 123
                            bool = true
                            object {
                                bar = "baz"
                            }
                        }
                    }
                }
            }
        }
        task "bar" {
            driver = "raw_exec"
            config {
                command = "bash"
                args    = ["-c", "echo hi"]
            }
            resources {
                network {
                    mbits = 10
                }
            }
        }
    }
}
