#!/usr/bin/env bash

# set up protoc so that we can use it for protobuf generation
PROTOC_ZIP=protoc-3.6.1-linux-x86_64.zip
curl -OL https://github.com/google/protobuf/releases/download/v3.6.1/$PROTOC_ZIP
sudo unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
rm -f $PROTOC_ZIP

