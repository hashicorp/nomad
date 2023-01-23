import parameterized from './default_jobs/parameterized';

export default [
  // {
  //   id: "nomad/job-templates/hello-world",
  //   keyValues: [
  //     {
  //       key: "template",
  //       value: "beep boop lol"
  //     },
  //     {
  //       key: "description",
  //       value: "A CODE-DRIVEN simple job that runs a single task on a single node.",
  //     },     
  //   ]
  // },
  {
    id: "nomad/job-templates/parameterized-job",
    keyValues: [
      {
        key: "template",
        value: parameterized
      },
      {
        key: "description",
        value: "TODO",
      },     
    ]
  },
  {
    id: "nomad/job-templates/service-discovery",
    keyValues: [
      {
        key: "template",
        value: `job "service-discovery-example" {
          datacenters = ["dc1"]
        
          group "cache" {
            network {
              port "db" {
                to = 6379
              }
            }
            service {
              // TODO: COMMENT
              provider = "nomad"
              name = "redis"
              // TODO: COMMENT
              port = "db"
              // TODO: COMMENT
              check {
                name = "up"
                type = "tcp"
                interval = "5s"
                timeout = "1s"
              }
            }
        
            task "redis" {
              driver = "docker"
        
              config {
                image          = "redis:7"
                ports          = ["db"]
                auth_soft_fail = true
              }
        
              resources {
                cpu    = 500
                memory = 256
              }
            }
          }
        }
        
        `
      },
      {
        key: "description",
        value: "TODO",
      },     
    ]
  }

];