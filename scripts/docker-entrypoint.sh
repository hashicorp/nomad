#!/usr/bin/env ash

case "$1" in
  "agent" )
    if [[ -z "${NOMAD_SKIP_DOCKER_IMAGE_WARN}" ]]
    then
      echo "====================================================================================="
      echo "!! Running Nomad clients inside Docker containers is not supported.                !!"
      echo "!! Refer to https://www.nomadproject.io/s/nomad-in-docker for more information.    !!"
      echo "!! Set the NOMAD_SKIP_DOCKER_IMAGE_WARN environment variable to skip this warning. !!"
      echo "====================================================================================="
      echo ""
      sleep 2
    fi
esac

exec nomad "$@"
