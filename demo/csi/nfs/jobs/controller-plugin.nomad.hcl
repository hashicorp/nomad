# controller plugins create and manage volumes and associated nomad state.
# this one just creates a folder in the dir that the nfs server exports.
job "controller" {
  type = "service"
  group "controller" {
    # most controller plugins should be host-independent, but for this demo,
    # both NFS server and controller plugin use this host volume directly.
    volume "host-nfs" {
      type      = "host"
      source    = "host-nfs"
      read_only = false
    }
    network {
      port "nfs" {
        to     = 2049
        static = 2049
      }
    }

    task "controller" {
      driver = "docker"
      config {
        image = "democraticcsi/democratic-csi:v1.8.3"
        args = [
          "--csi-version=1.2.0",
          "--csi-name=org.democratic-csi.nfs",
          "--driver-config-file=${NOMAD_TASK_DIR}/driver-config.yaml",
          "--log-level=debug",
          "--csi-mode=controller",
          "--server-socket=${CSI_ENDPOINT}", # /csi/csi.sock
        ]
      }
      csi_plugin {
        id   = "org.democratic-csi.nfs" # must match --csi-name
        type = "controller"             # --csi-mode
      }
      template {
        destination = "${NOMAD_TASK_DIR}/driver-config.yaml"
        data = yamlencode({
          driver = "nfs-client"
          # ex: https://github.com/democratic-csi/democratic-csi/blob/v1.8.3/examples/nfs-client.yaml
          # source: https://github.com/democratic-csi/democratic-csi/blob/v1.8.3/src/driver/controller-nfs-client/index.js
          nfs = {
            # these are added as Context to volumes that get created,
            # for the node plugin to use later for mounting.
            shareHost     = "{{ env `NOMAD_IP_nfs` }}"
            shareBasePath = "/srv/nfs" # match what NFS exports

            # volume directories will be created in here with these perms.
            controllerBasePath = "/storage" # match volume_mount destination
            dirPermissionsMode = "0777"
            dirPermissions     = "root"
            dirPermissions     = "root"
          }
        })
      }
      volume_mount {
        volume      = "host-nfs"
        destination = "/storage"
      }
    }

    task "nfs" {
      driver = "docker"
      config {
        image      = "atlassian/nfs-server-test:2.1"
        ports      = ["nfs"]
        privileged = true
        mount { # because can't template directly into an /absolute/path
          type     = "bind"
          source   = "local/exports"
          target   = "/etc/exports"
          readonly = true
        }
      }
      template {
        destination = "local/exports"
        data        = "/srv/nfs *(rw,sync,no_subtree_check,no_auth_nlm,insecure,no_root_squash)"
      }
      volume_mount {
        volume      = "host-nfs"
        destination = "/srv/nfs"
      }
    }
  }
}
