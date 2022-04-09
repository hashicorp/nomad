#!/usr/bin/env bash

# Install NVM for simple node.js version management
curl -sSL --fail -o- https://raw.githubusercontent.com/creationix/nvm/v0.36.0/install.sh | bash

# This enables NVM without a logout/login
export NVM_DIR="${HOME}/.nvm"
# shellcheck source=/dev/null
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm

# Install Node, Ember CLI, and Phantom for UI development
# Use exact full version version (e.g. not 12) for reproducibility purposes
nvm install 12.22.10
nvm alias default 12.22.10
npm install -g ember-cli

# Install Yarn for front-end dependency management
curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version 1.22.5
