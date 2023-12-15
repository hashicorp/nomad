# Container Storage Interface (CSI) examples

This directory contains examples of registering CSI plugin jobs and volumes
for those plugins.

Contributions are welcome but demos are *not supported* by the core Nomad
development team. Please tag demo authors when filing issues about CSI demos.

### Contributing

If you'd like to contribute an example, open a PR with a folder for the plugin
in this directory. This folder should include:

* A `README.md` with any added instructions a user might need to use the
  plugin. Please include a link to the CSI plugin's source repository.
  * Add an `Author: @<Github Username>` field at the top so you can be tagged
    on issues.
* A Nomad job file for the plugin.
* A [volume specification](https://developer.hashicorp.com/nomad/docs/commands/volume/register#volume-specification) file.
