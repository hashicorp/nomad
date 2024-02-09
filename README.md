# Apply ONMO changes and compile Nomad

Follow these step for easier patching a new version of nomad

Clone our repo:

```bash
git clone git@github.com:B2tGame/nomad.git
cd nomad
```

Find the wanted release branch in HC nomad, example: *`release/1.6.2`*

Sync the remote repo

```bash
git remote add hcnomad https://github.com/hashicorp/nomad.git
git fetch upstream release/1.6.2
git checkout -b v1.6.2-onmo hcnomad/release/1.6.2
git push --set-upstream origin v1.6.2-onmo
git remote remove hcnomad
```

Find the commit hash from the previous release in our repo and Apply the patch with cherry-pick and fix the merge conflict.

```bash
git cherry-pick de6a2d40e9b25ae4c78b0eaf8f88100244ad48ee
```

Check the required Go version in `go.mod` and install. Then export to PATH

```bash
sudo apt update
sudo apt install gcc gcc-8-aarch64-linux-gnu
npm install -g yarn # required for Ember
# install go
https://go.dev/doc/install

export PATH=$PATH:/usr/local/go/bin/:$(/usr/local/go/bin/go env GOPATH)/bin
```

Ember uses a lot of heap memory, increase it by

```bash
sudo sysctl -w vm.max_map_count=655300
```

For testing nomad, run the following

```bash
# these are needed to be ran only one time
make bootstrap
make dev-ui # rerun this command if you made change in the nomad ui

# This needs to be re-run everytime you make change to nomad code
./dev-build.sh
```

Compile nomad and uploaded it to s3

Prerequisite: Install the following package to enable windows builds
```bash
sudo apt-get install gcc-mingw-w64
```

To compile:

```bash
./compile-nomad # the output will be in ./pkg/linux_amd64/nomad
```

```bash
s3 cp ./pkg/linux_amd64/nomad s3://docs.appland-stream.com/streaming/bin/nomad/v1.6.2/nomad.amd64
```