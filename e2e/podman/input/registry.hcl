# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job stands up a private container registry for use in e2e tests.
# In a post start task we then upload some default images for convenience.
#
# <address>:<port>/docker.io/library/bash_auth_basic:private
#
# Note that the <address>:<port> is dynamic and can be found using NSD.
# Note that credentials are required (e.g. podman login), and are specific to
# each image, e.g. "auth_basic_user" and "auth_basic_pass".
#
# To add a new username/password credential, run this container command
# podman run --rm --entrypoint htpasswd registry:2.7.0 -Bbn <username> <password>
# and add <username>:<hash> to the local/auth.txt file template below.

job "registry" {
  type = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "registry-server" {

    update {
      min_healthy_time = "4s"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    network {
      mode = "host"
      port "registryhttp" {}
    }

    service {
      provider = "nomad"
      name     = "registry"
      port     = "registryhttp"
      check {
        name     = "registry-http"
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "3s"
      }
    }

    task "registry" {
      driver = "podman"

      template {
        data        = <<EOH
e2euser:$2y$05$QpRvGkM/CMG.AG/G7Uh6guULMIlv1ZvjwfPa6dNjdkH.fhTzcpLDC
auth_basic_user:$2y$05$b/lpKjGJhVMdgbpu1hxe0eAGegeHFrsWXH9g0JEO2gcWzPNgvesby
auth_static_user:$2y$05$ZDOhbzsNe9pCcR0NslV72.gTrRLwI.05tq5yJMtFkD2LSS.G0wAYe
auth_helper_user:$2y$05$sY4qctfzsjIhNyPD.zBEVumP0l6V5gU1f6GEThvHQ1cwupS8rogtu
        EOH
        destination = "local/auth.txt"
      }

      config {
        image          = "docker.io/library/registry:2"
        auth_soft_fail = true
        ports          = ["registryhttp"]
        network_mode   = "host"
      }

      env {
        REGISTRY_HTTP_ADDR           = "${NOMAD_ADDR_registryhttp}"
        REGISTRY_AUTH                = "htpasswd"
        REGISTRY_AUTH_HTPASSWD_REALM = "Registry Realm"
        REGISTRY_AUTH_HTPASSWD_PATH  = "local/auth.txt"
      }

      resources {
        cpu    = 250
        memory = 200
      }
    }


    task "registry-preload" {
      user   = "root"
      driver = "raw_exec"

      lifecycle {
        hook    = "poststart"
        sidecar = false
      }

      template {
        data        = <<EOH
{
  "auths": {
    "{{- env "NOMAD_ADDR_registryhttp" -}}": {
      "auth": "ZTJldXNlcjplMmVwYXNzd29yZA=="
    }
  }
}
        EOH
        destination = "local/auth.json"
      }

      template {
        data        = <<EOH
set -xeuo pipefail
podman pull docker.io/library/bash:5
podman push --tls-verify=false --authfile=local/auth.json docker.io/library/bash:5 {{env "NOMAD_ADDR_registryhttp" -}}/docker.io/library/bash_auth_basic:private
podman push --tls-verify=false --authfile=local/auth.json docker.io/library/bash:5 {{env "NOMAD_ADDR_registryhttp" -}}/docker.io/library/bash_auth_static:private
podman push --tls-verify=false --authfile=local/auth.json docker.io/library/bash:5 {{env "NOMAD_ADDR_registryhttp" -}}/docker.io/library/bash_auth_helper:private
        EOH
        destination = "local/script.sh"
      }

      config {
        command = "bash"
        args    = ["local/script.sh"]
      }

      resources {
        cpu    = 200
        memory = 100
      }
    }
  }
}
