# End to End Tests

This package contains integration tests.

The `terraform` folder has provisioning code to spin up a Nomad cluster on AWS.
The tests work with the `NOMAD_ADDR` environment variable which can be set
either to a local dev Nomad agent or a Nomad client on AWS.

## Local Development

The workflow when developing end to end tests locally is to run the
provisioning step described below once, and then run the tests as described
below.

When making local changes, use `./bin/update $(which nomad) /usr/local/bin/nomad`
and `./bin/run sudo systemctl restart nomad` to destructively modify the
provisioned cluster.

## Provisioning Test Infrastructure on AWS

You'll need Terraform and AWS credentials (`AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`) to setup AWS instances on which e2e tests
will run. See the [README](https://github.com/hashicorp/nomad/blob/master/e2e/terraform/README.md)
for details. The number of servers and clients is configurable, as is
the configuration file for each client and server.

## Provisioning e2e Framework Nomad Cluster

You can use the Terraform output from the previous step to generate a
provisioning configuration file for the e2e framework.

```sh
# from the ./e2e/terraform directory
terraform output provisioning | jq . > ../provisioning.json
```

By default the `provisioning.json` will include a `nomad_sha` field
for each node. You can edit this file to change the version of Nomad
you want to deploy. Because each node has its own value, you can
create cluster of mixed versions. The provisioning framework accepts
any of the following options:

- `nomad_sha`: This is a Nomad binary identified by its full commit SHA that's
  stored in a shared s3 bucket that Nomad team developers can access. That
  commit SHA can be from any branch that's pushed to remote.  (Ex.
  `"nomad_sha": "0b6b475e7da77fed25727ea9f01f155a58481b6c"`)
- `nomad_local_binary`: This is a path to a Nomad binary on your own host.
  (Ex. `"nomad_local_binary": "/home/me/nomad"`)
- `nomad_version`: This is a version number of Nomad that's been released to
  HashiCorp. (Ex. `"nomad_version": "0.10.2"`)

You can pass the following flags to `go test` to override the values
in `provisioning.json` for all nodes:

- `-nomad.local_file=string`: provision this specific local binary of Nomad
- `-nomad.sha=string`: provision this specific sha from S3
- `-nomad.version=string`: provision this version from
  [releases.hashicorp.com](https://releases.hashicorp.com/nomad)

Deploy Nomad to the cluster:

```sh
# from the ./e2e/terraform directory, set your client environment
$(terraform output environment)

cd ..
go test -v . -provision.terraform ./provisioning.json -skipTests
```

## Running

After completing the provisioning step above, you can set the client
environment for `NOMAD_ADDR` and run the tests as shown below:

```sh
# from the ./e2e/terraform directory, set your client environment
# if you haven't already
$(terraform output environment)

cd ..
go test -v .
```

If you want to run a specific test, you'll need to regex-escape some of the
test's name so that the test runner doesn't skip over framework struct method
names in the full name of the tests:

```sh
 go test -v . -run 'TestE2E/Consul/\*consul\.ScriptChecksE2ETest/TestGroup'
 ```
