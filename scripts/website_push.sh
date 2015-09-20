#!/bin/bash

# Set the tmpdir
if [ -z "$TMPDIR" ]; then
  TMPDIR="/tmp"
fi

# Create a temporary build dir and make sure we clean it up. For
# debugging, comment out the trap line.
DEPLOY=`mktemp -d $TMPDIR/nomad-www-XXXXXX`
trap "rm -rf $DEPLOY" INT TERM EXIT

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Copy into tmpdir
cp -R $DIR/website/ $DEPLOY/

# Change into that directory
pushd $DEPLOY &>/dev/null

# Ignore some stuff
touch .gitignore
echo ".sass-cache" >> .gitignore
echo "build" >> .gitignore

# Add everything
git init -q .
git add .
git commit -q -m "Deploy by $USER"

git remote add heroku git@heroku.com:nomad-www.git
git push -f heroku master

# Go back to our root
popd &>/dev/null
