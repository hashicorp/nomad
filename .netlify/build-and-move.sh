#!/usr/bin/env bash

cd website
bundle install
bundle exec middleman build

cd ../ui/
npm install -g yarn
yarn
ember build
mkdir -p ../website/build/ui

mv dist/* ../website/build/ui/
