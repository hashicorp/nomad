#!/bin/bash

set -o errexit
set -o nounset
set +x

usage() {
    cat <<EOF
Usage: provision.sh [options...]
Options (use one of the following):
 --nomad_sha SHA          full git sha to install from S3
 --nomad_version VERSION  release version number (ex. 0.12.4+ent)
 --nomad_binary FILEPATH  path to file on host

Options for configuration:
 --config_profile FILEPATH  path to config profile directory
 --role ROLE                role within config profile directory
 --index INDEX              count of instance, for profiles with per-instance config
 --nostart                  do not start or restart Nomad
 --enterprise               if nomad_sha is passed, use the ENT version
 --nomad_acls               write Nomad ACL configuration
 --autojoin                 the AWS ConsulAutoJoin tag value

EOF

    exit 2
}


INSTALL_DIR=/usr/local/bin
INSTALL_PATH="${INSTALL_DIR}/nomad"
PLATFORM=linux_amd64
START=1
install_fn=

NOMAD_PROFILE=
NOMAD_ROLE=
NOMAD_INDEX=
BUILD_FOLDER="builds-oss"
CONSUL_AUTOJOIN=
ACLS=0

install_from_s3() {
    # check that we don't already have this version
    if [ "$(command -v nomad)" ]; then
        nomad -version | grep -q "${NOMAD_SHA}" \
            && echo "$NOMAD_SHA already installed" && return
    fi

    S3_URL="s3://nomad-team-dev-test-binaries/${BUILD_FOLDER}/nomad_${PLATFORM}_${NOMAD_SHA}.tar.gz"
    aws s3 cp --quiet "$S3_URL" nomad.tar.gz
    sudo tar -zxvf nomad.tar.gz -C "$INSTALL_DIR"
    set_ownership
}

install_from_uploaded_binary() {
    # we don't need to check for reinstallation here because we do it at the
    # user's end so that we're not copying it up if we don't have to
    sudo cp "$NOMAD_UPLOADED_BINARY" "$INSTALL_PATH"
    set_ownership
}

install_from_release() {
    # check that we don't already have this version
    if [ "$(command -v nomad)" ]; then
        nomad -version | grep -v 'dev' | grep -q "${NOMAD_VERSION}" \
            && echo "$NOMAD_VERSION already installed" && return
    fi

    RELEASE_URL="https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_${PLATFORM}.zip"
    curl -sL --fail -o /tmp/nomad.zip "$RELEASE_URL"
    sudo unzip -o /tmp/nomad.zip -d "$INSTALL_DIR"
    set_ownership
}

set_ownership() {
    sudo chmod 0755 "$INSTALL_PATH"
    sudo chown root:root "$INSTALL_PATH"
}

sym() {
    find "$1" -maxdepth 1 -type f -name "$2" 2>/dev/null \
        | sudo xargs -I % ln -fs % "$3"
}

install_config_profile() {

    if [ -d /tmp/custom ]; then
        rm -rf /opt/config/custom
        sudo mv /tmp/custom /opt/config/
    fi

    # we're removing the whole directory and recreating to avoid
    # any quirks around dotfiles that might show up here.
    sudo rm -rf /etc/nomad.d
    sudo rm -rf /etc/consul.d
    sudo rm -rf /etc/vault.d

    sudo mkdir -p /etc/nomad.d
    sudo mkdir -p /etc/consul.d
    sudo mkdir -p /etc/vault.d

    sym "${NOMAD_PROFILE}/nomad/" '*' /etc/nomad.d
    sym "${NOMAD_PROFILE}/consul/" '*' /etc/consul.d
    sym "${NOMAD_PROFILE}/vault/" '*' /etc/vault.d

    if [ -n "$NOMAD_ROLE" ]; then
        sym "${NOMAD_PROFILE}/nomad/${NOMAD_ROLE}/" '*' /etc/nomad.d
        sym "${NOMAD_PROFILE}/consul/${NOMAD_ROLE}/" '*' /etc/consul.d
        sym "${NOMAD_PROFILE}/vault/${NOMAD_ROLE}/" '*' /etc/vault.d
    fi
    if [ -n "$NOMAD_INDEX" ]; then
        sym "${NOMAD_PROFILE}/nomad/${NOMAD_ROLE}/indexed/" "*${NOMAD_INDEX}*" /etc/nomad.d
        sym "${NOMAD_PROFILE}/consul/${NOMAD_ROLE}/indexed/" "*${NOMAD_INDEX}*" /etc/consul.d
        sym "${NOMAD_PROFILE}/vault/${NOMAD_ROLE}/indexed/" "*${NOMAD_INDEX}*" /etc/vault.d
    fi

    if [ $ACLS == "1" ]; then
        sudo ln -fs /opt/config/shared/nomad-acl.hcl /etc/nomad.d/acl.hcl
    fi
}

update_consul_autojoin() {
    sudo sed -i'' -e "s|tag_key=ConsulAutoJoin tag_value=auto-join|tag_key=ConsulAutoJoin tag_value=${CONSUL_AUTOJOIN}|g" /etc/consul.d/*.json
}

while [[ $# -gt 0 ]]
do
opt="$1"
    case $opt in
        --nomad_sha)
            if [ -z "$2" ]; then echo "Missing sha parameter"; usage; fi
            NOMAD_SHA="$2"
            install_fn=install_from_s3
            shift 2
            ;;
        --nomad_release | --nomad_version)
            if [ -z "$2" ]; then echo "Missing version parameter"; usage; fi
            NOMAD_VERSION="$2"
            install_fn=install_from_release
            shift 2
            ;;
        --nomad_binary)
            if [ -z "$2" ]; then echo "Missing file parameter"; usage; fi
            NOMAD_UPLOADED_BINARY="$2"
            install_fn=install_from_uploaded_binary
            shift 2
            ;;
        --config_profile)
            if [ -z "$2" ]; then echo "Missing profile parameter"; usage; fi
            NOMAD_PROFILE="/opt/config/${2}"
            shift 2
            ;;
        --role)
            if [ -z "$2" ]; then echo "Missing role parameter"; usage; fi
            NOMAD_ROLE="$2"
            shift 2
            ;;
        --index)
            if [ -z "$2" ]; then echo "Missing index parameter"; usage; fi
            NOMAD_INDEX="$2"
            shift 2
            ;;
        --autojoin)
            if [ -z "$2" ]; then ehco "Missing autojoin parameter"; usage; fi
            CONSUL_AUTOJOIN="$2"
            shift 2
            ;;
        --nostart)
            # for initial packer builds, we don't want to start Nomad
            START=0
            shift
            ;;
        --enterprise)
            BUILD_FOLDER="builds-ent"
            shift
            ;;
        --nomad_acls)
            ACLS=1
            shift
            ;;
        *) usage ;;
    esac
done

# call the appropriate installation function
if [ -n "$install_fn" ]; then
    $install_fn
fi
if [ -n "$NOMAD_PROFILE" ]; then
    install_config_profile
fi

if [ -n "$CONSUL_AUTOJOIN" ]; then
    update_consul_autojoin
fi

if [ $START == "1" ]; then
    if [ "$NOMAD_ROLE" == "server" ]; then
        sudo systemctl restart vault
    fi
    sudo systemctl restart consul
    sudo systemctl restart nomad
fi
