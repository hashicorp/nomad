job "spark-history-server" {
  datacenters = ["dc1"]
  type = "service"

  group "server" {
    count = 1

    task "history-server" {
      driver = "docker"
      
      config {
        image = "barnardb/spark"
        command = "/spark/spark-2.1.0-bin-nomad/bin/spark-class"
        args = [ "org.apache.spark.deploy.history.HistoryServer" ]
        port_map {
          ui = 18080
        }
        network_mode = "host"
      }

      env {
        "SPARK_HISTORY_OPTS" = "-Dspark.history.fs.logDirectory=hdfs://hdfs.service.consul/spark-events/"
        "SPARK_PUBLIC_DNS"   = "spark-history.service.consul"
      }

      resources {
        cpu    = 500
        memory = 500
        network {
          mbits = 250
          port "ui" {
            static = 18080
          }
        }
      }

      service {
        name = "spark-history"
        tags = ["spark", "ui"]
        port = "ui"
      }
    }

  }
}
