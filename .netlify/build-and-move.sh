#!/usr/bin/env bash

# Build the static web site in website/build
bundle install
bundle exec middleman build

# Build the UI and move it to website/build/ui
cd ../ui/
# npm install -g yarn ember-cli
# yarn
# ember build
# mkdir -p ../website/build/ui

# mv dist/* ../website/build/ui/

cd ../

echo "Determining which _redirects file to use based on head branch $HEAD"

if [[ "$HEAD" =~ ^.-ui\/ ]]; then
    echo "Using the _redirects file for UI"
    cp .netlify/ui-redirects website/build/_redirects
else
    echo "Using the default _redirects file"
    cp .netlify/default-redirects website/build/_redirects
fi
