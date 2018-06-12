#!/usr/bin/env bash

# Install NVM for simple node.js version management
wget -qO- https://raw.githubusercontent.com/creationix/nvm/v0.33.11/install.sh | bash

# This enables NVM without a logout/login
export NVM_DIR="/home/vagrant/.nvm"
# shellcheck source=/dev/null
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm

# Install Node, Ember CLI, and Phantom for UI development
nvm install 8.11.2
nvm alias default 8.11.2
npm install -g ember-cli

# Install Yarn for front-end dependency management
curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version 1.7.0

# Install Chrome for running tests (in headless mode)
wget -qO- - https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
sudo sh -c 'echo "deb https://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list'
sudo apt-get update
sudo apt-get install -y google-chrome-stable
