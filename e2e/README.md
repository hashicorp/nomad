End to End Tests
================

This package contains integration tests. 

The `terraform` folder has provisioning code to spin up a Nomad cluster on AWS. The tests work with the `NOMAD_ADDR` environment variable which can be set either to a local dev Nomad agent or a Nomad client on AWS. 

Local Development
=================
The workflow when developing end to end tests locally is to run the provisioning step described below once, and then run the tests as described below. 

Provisioning
============
You'll need AWS credentials (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`) to create the Nomad cluster. See the [README](https://github.com/hashicorp/nomad/blob/master/e2e/terraform/README.md) for details. The number of servers and clients is configurable, as is the configuration file for each client and server.

Running
===========
After completing the provisioning step above, you should see CLI output showing the IP addresses of Nomad client machines. To run the tests, set the NOMAD_ADDR variable to one of the client IPs.

```
$ NOMAD_ADDR=<> $NOMAD_E2E=1 go test -v
```
