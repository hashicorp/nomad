# Cross-Compile Nomad

## Install Cross-Compile Tools
In order to cross-compile Nomad from your MacOS Darwin (Silicon), you need cross-compilation tools.

You can get these from [homebrew-macos-cross-toolchains](https://github.com/messense/homebrew-macos-cross-toolchains).

You will use `arm-unknown-linux-gnueabihf` to cross-compile.

```bash
brew tap messense/macos-cross-toolchains
brew install messense/macos-cross-toolchains/arm-unknown-linux-gnueabihf
```

Verify that you have `arm-unknown-linux-gnueabihf-gcc` by running the following in bash.

```bash
which arm-unknown-linux-gnueabihf-gcc
```

## Make the Nomad Executable

This repo uses a modified `GNUMakefile` to cross-compile. We export the CC variable to use `arm-unknown-linux-gnueabihf-gcc` for compilation.

Go ahead and build a nomad release by running the following in the **root of this repository**.

```bash
make
```

Your file will be in `pkg/linux_arm/nomad`.

# [Optional] Steps to Create Service

## Create the Nomad Configuration directory

In your `armv6l` host (i.e. Raspberry Pi), run the following.

```bash
sudo mkdir /etc/nomad.d
sudo touch /etc/nomad.d/nomad.env /etc/nomad.d/nomad.hcl
```

Your Nomad configuration will live in `nomad.hcl`. 

Refer to the [Nomad docs](https://developer.hashicorp.com/nomad/docs/configuration).

## Copy Nomad Executable
Copy your Nomad executable file to the server. Example using `scp` (fill sections):

```bash
NOMAD_EXE_PATH=...
SERVER_USER=...
SERVER_HOST=...
scp "${NOMAD_EXE_PATH}" "${SERVER_USER}@${SERVER_HOST}:/usr/bin/nomad"
```

## Create Nomad System Service
You can create the Nomad system service with the following.

```bash
sudo touch /lib/systemd/system/nomad.service
sudo nano /lib/systemd/system/nomad.service
```

You can copy [this](nomad.service) into the service file.

Finally, start the service.

```bash
sudo service nomad start
```
Optionally, enable the service at system start.

```bash
sudo systemctl enable nomad.service
```

Email me with any questions at contact@kenji.us.