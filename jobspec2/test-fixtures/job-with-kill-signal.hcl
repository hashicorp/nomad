job "foo" {
  task "bar" {
    driver      = "docker"
    kill_signal = "SIGQUIT"

    config {
      image = "hashicorp/image"
    }
  }
}
