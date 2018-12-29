## Running the Web UI in development mode against a production Nomad cluster

:warning: **Running the Web UI in development mode is only necessary when debugging
issues. Unless you are debugging an issue, please only use the Web UI contained
in the Nomad binary.** :warning:

The production Web UI concatenates and minifies JavaScript and CSS. This can make errors
cryptic or useless. In development mode, files are as expected and stack traces are useful.

Debugging Web UI issues with the Web UI in development mode is done in three steps:

  1. Cloning the Nomad Repo
  2. Setting up your environment (or using Vagrant)
  3. Serving the Web UI locally while proxying to the production Nomad cluster

### Cloning the Nomad Repo

The Web UI is part of the same repo as Nomad itself. Clone the repo
[using Github](https://help.github.com/articles/cloning-a-repository/).

### Setting up your environment

The [Web UI README](README.md) includes a list of software prerequisites and instructions
for running the UI locally or with the Vagrant VM.

### Serving the Web UI locally while proxying to the production Nomad cluster

Serving the Web UI is done with a single command in the `/ui` directory.

  - **Local:** `ember serve`
  - **Vagrant:** `ember serve --watch polling --port 4201`

However, this will use the [Mirage fixtures](http://www.ember-cli-mirage.com/) as a backend.
To use your own Nomad cluster as a backend, use the proxy option.

  - **Local:** `ember serve --proxy https://demo.nomadproject.io`
  - **Vagrant:** `ember serve --watch polling --port 4201 --proxy https://demo.nomadproject.io`

The Web UI will now be accessible from your host machine.

  - **Local:** [http://localhost:4200](http://localhost:4200)
  - **Vagrant:** [http://localhost:4201](http://localhost:4201)
