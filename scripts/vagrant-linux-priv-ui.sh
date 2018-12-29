#!/usr/bin/env bash

# Install NVM for simple node.js version management
wget -qO- https://raw.githubusercontent.com/creationix/nvm/v0.33.2/install.sh | bash

# This enables NVM without a logout/login
export NVM_DIR="/home/vagrant/.nvm"
# shellcheck source=/dev/null
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm

# Install Node, Ember CLI, and Phantom for UI development
nvm install 6.11.0
nvm alias default 6.11.0
npm install -g ember-cli phantomjs-prebuilt

# Install Yarn for front-end dependency management
curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version 0.24.6
export PATH="$HOME/.yarn/bin:\$PATH"
