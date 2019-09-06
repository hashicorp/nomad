#!/usr/bin/env bash

set -o errexit

bash ./scripts/travis-consul.sh
bash ./scripts/travis-vault.sh
