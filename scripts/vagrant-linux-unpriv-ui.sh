#!/usr/bin/env bash

# Install NVM for simple node.js version management
curl -sSL --fail -o- https://raw.githubusercontent.com/creationix/nvm/v0.33.11/install.sh | bash

# This enables NVM without a logout/login
export NVM_DIR="/home/vagrant/.nvm"
# shellcheck source=/dev/null
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm

# Install Node, Ember CLI, and Phantom for UI development
nvm install 10.15.3
nvm alias default 10.15.3
npm install -g ember-cli

# Install Yarn for front-end dependency management
curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version 1.15.2
