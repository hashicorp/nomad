#!/usr/bin/env bash

brew upgrade golang
bash ./scripts/travis-consul.sh
bash ./scripts/travis-vault.sh
