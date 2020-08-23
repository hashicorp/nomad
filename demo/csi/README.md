# Container Storage Interface (CSI) examples

This directory contains examples of registering CSI plugin jobs and volumes
for those plugins.

### Contributing

If you'd like to contribute an example, open a PR with a folder for the plugin
in this directory. This folder should include:

* A `README.md` with any added instructions a user might need to use the
  plugin. Please include a link to the CSI plugin's source repository.
* A Nomad job file for the plugin.
* A [volume specification](https://www.nomadproject.io/docs/commands/volume/register#volume-specification) file.
