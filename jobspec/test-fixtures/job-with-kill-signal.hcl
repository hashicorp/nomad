job "example" {
    task "webservice" {
      kill_signal = "SIGINT"
        driver = "docker"
        config
        {
          image =  "hashicorp/image"
        }
    }

}

