# outputs used for E2E testing and provisioning

output "provisioning" {
  description = "output to a file to be use w/ E2E framework -provision.terraform"
  value = jsonencode(
    {
      "servers" : [for server in aws_instance.server.* :
        {
          "runner" : {
            "key" : abspath(module.keys.private_key_filepath),
            "user" : "ubuntu",
            "host" : "${server.public_ip}",
            "port" : 22
          },
          "deployment" : {
            "nomad_sha" : var.nomad_sha,
            "platform" : "linux_amd64",
            "remote_binary_path" : "/usr/local/bin/nomad",
            "bundles" : [
              {
                "source" : abspath("./shared"),
                "destination" : "/ops/shared"
              }
            ],
            "steps" : [
              "sudo chmod +x /ops/shared/config/provision-server.sh",
              "sudo /ops/shared/config/provision-server.sh aws ${var.server_count} 'indexed/server-${index(aws_instance.server, server)}.hcl'"
            ],
          }
        }
      ],
      "clients" : concat([for client in aws_instance.client_linux.* :
        {
          "runner" : {
            "key" : abspath(module.keys.private_key_filepath),
            "user" : "ubuntu",
            "host" : "${client.public_ip}",
            "port" : 22
          },
          "deployment" : {
            "nomad_sha" : var.nomad_sha,
            "platform" : "linux_amd64",
            "remote_binary_path" : "/usr/local/bin/nomad",
            "bundles" : [
              {
                "source" : abspath("./shared"),
                "destination" : "/ops/shared"
              }
            ],
            "steps" : [
              "sudo chmod +x /ops/shared/config/provision-client.sh",
              "sudo /ops/shared/config/provision-client.sh aws 'indexed/client-${index(aws_instance.client_linux, client)}.hcl'"
            ],
          }
        }
        ],
        [for client in aws_instance.client_windows.* :
          {
            "runner" : {
              "key" : abspath(module.keys.private_key_filepath),
              "user" : "Administrator",
              "host" : "${client.public_ip}",
              "port" : 22
            },
            "deployment" : {
              "nomad_sha" : var.nomad_sha,
              "platform" : "windows_amd64",
              # need to use the / here for golang filepath handling to work
              # on the Unix test runner environment
              "remote_binary_path" : "C:/opt/nomad.exe",
              "bundles" : [
                {
                  "source" : abspath("./shared"),
                  "destination" : "C:/ops/shared"
                }
              ],
              "steps" : [
                "& C:\\ops\\shared\\config\\provision-windows-client.ps1 -Cloud aws -Index 1"
              ]
            }
          }
      ])
  })
}
