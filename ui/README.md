# Nomad UI

The official Nomad UI.

## Prerequisites

This is an [ember.js](https://emberjs.com/) project, and you will need the following tools installed on your computer.

* [Node.js](https://nodejs.org/)
* [Yarn](https://yarnpkg.com)
* [Ember CLI](https://ember-cli.com/)
* [PhantomJS](http://phantomjs.org/) (for running tests)

## Installation

The Nomad UI gets cloned along with the rest of Nomad. To install dependencies, do the following from the root of the Nomad project:

* `cd ui`
* `yarn`

## Running / Development

First, make sure nomad is running. The UI, in development mode, runs independently from Nomad, so this could be an official release or a dev branch. Likewise, Nomad can be running in server mode or dev mode. As long as the API is accessible, the UI will work as expected.

* `ember serve --proxy=localhost:4646`
* Visit your app at [http://localhost:4200](http://localhost:4200).

**Note:** The proxy address needs to be the address Nomad is bound to. `localhost:4646` is the default and is shown here for convenience.

**Note:** When running Nomad with Vagrant, make sure the process is bound to `0.0.0.0` so your host machine can properly access it.

### Running Tests

Nomad UI tests can be run independently of Nomad golang tests.

* `ember test` (single run, headless browser)
* `ember test --server` (watches for changes, runs in a full browser)

### Building

Typically `make release` or `make dev-ui` will be the desired build workflow, but in the event that build artifacts need to be inspected, `ember build` will output compiled files in `ui/dist`.

* `ember build` (development)
* `ember build --environment production` (production)

### Releasing

Nomad UI releases are in lockstep with Nomad releases and are integrated into the `make release` toolchain.
