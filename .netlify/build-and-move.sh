#!/usr/bin/env bash

cd website
bundle install
bundle exec middleman build

cd ../ui/
npm install -g yarn ember-cli
yarn
ember build
mkdir -p ../website/build/ui

mv dist/* ../website/build/ui/

cd ../

if [[ "$BRANCH" =~ ^.-ui\/ ]]; then
    cp .netlify/ui-redirects website/build/_redirects
else
    cp .netlify/default-redirects website/build/_redirects
fi
