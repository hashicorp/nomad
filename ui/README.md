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

```
$ cd ui
$ yarn
```

## Running / Development

First, make sure nomad is running. The UI, in development mode, runs independently from Nomad, so this could be an official release or a dev branch. Likewise, Nomad can be running in server mode or dev mode. As long as the API is accessible, the UI will work as expected.

* `ember serve`
* Visit your app at [http://localhost:4200](http://localhost:4200).

## Running / Development with Vagrant

All necessary tools for UI development are installed as part of the Vagrantfile. This is primarily to make it easy to build the UI from source while working on Nomad. Due to the filesystem requirements of [Broccoli](http://broccolijs.com/) (which powers Ember CLI), it is strongly discouraged to use Vagrant for developing changes to the UI.

That said, development with Vagrant is still possible, but the `ember serve` command requires two modifications:

* `--watch polling`: This allows the vm to notice file changes made in the host environment.
* `--port 4201`: The default port 4200 is not forwarded, since local development is recommended.

This makes the full command for running the UI in development mode in Vagrant:

```
$ ember serve --watch polling --port 4201
```

### Running Tests

Nomad UI tests can be run independently of Nomad golang tests.

* `ember test` (single run, headless browser)
* `ember test --server` (watches for changes, runs in a full browser)



### Linting

Linting should happen automatically in your editor and when committing changes, but it can also be invoked manually.

* `npm run lint:hbs`
* `npm run lint:js`
* `npm run lint:js -- --fix`

### Building

Typically `make release` or `make dev-ui` will be the desired build workflow, but in the event that build artifacts need to be inspected, `ember build` will output compiled files in `ui/dist`.

* `ember build` (development)
* `ember build --environment production` (production)

### Releasing

Nomad UI releases are in lockstep with Nomad releases and are integrated into the `make release` toolchain.

### Troubleshooting

#### The UI is running, but none of the API requests are working

By default (according to the `.embercli` file) a proxy address of `http://localhost:4646` is used. If you are running Nomad at a different address, you will need to override this setting when running ember serve: `ember serve --proxy http://newlocation:1111`.

#### Nomad is running in Vagrant, but I can't access the API from my host machine

Nomad binds to `127.0.0.1:4646` by default, which is the loopback address. Try running nomad bound to `0.0.0.0`: `bin/nomad -bind 0.0.0.0`.

Ports also need to be forwarded in the Vagrantfile. 4646 is already forwarded, but if a port other than the default is being used, that port needs to be added to the Vagrantfile and `vagrant reload` needs to be run.
