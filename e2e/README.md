# End to End Tests

This package contains integration tests. Unlike tests alongside Nomad code,
these tests expect there to already be a functional Nomad cluster accessible
(either on localhost or via the `NOMAD_ADDR` env var).

The `terraform` folder has provisioning code to spin up a Nomad cluster on AWS.
The tests work with the `NOMAD_ADDR` environment variable which can be set
either to a local dev Nomad agent or a Nomad client on AWS.

The `NOMAD_E2E=1` environment variable must be set for these tests to run.

## Local Nomad Development

When developing tests locally, provisioning is not required when only the tests
change. See [`framework/doc.go`](framework/doc.go) for how to write tests.

When making changes to the Nomad agent itself, use `./bin/update $(which nomad)
/usr/local/bin/nomad` and `./bin/run sudo systemctl restart nomad` to
destructively modify the provisioned cluster.

## Provisioning Test Infrastructure on AWS

You'll need Terraform and AWS credentials (`AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`) to setup AWS instances on which e2e tests
will run. See the [README](https://github.com/hashicorp/nomad/blob/master/e2e/terraform/README.md)
for details. The number of servers and clients is configurable, as is
the configuration file for each client and server.

## Provisioning e2e Framework Nomad Cluster

You can use the Terraform output from the [previous step](https://github.com/hashicorp/nomad/blob/master/e2e/terraform/README.md)
to generate a provisioning configuration file for the e2e framework.

```sh
# from the ./e2e/terraform directory
terraform output provisioning | jq . > ../provisioning.json
```

By default the `provisioning.json` will not include the Nomad version
that will be deployed to each node. You can pass the following flags
to `go test` to set the version for all nodes:

- `-nomad.local_file=string`: provision this specific local binary of
  Nomad. This is a path to a Nomad binary on your own
  host. Ex. `-nomad.local_file=/home/me/nomad`
- `-nomad.sha=string`: provision this specific sha from S3. This is a
  Nomad binary identified by its full commit SHA that's stored in a
  shared s3 bucket that Nomad team developers can access. That commit
  SHA can be from any branch that's pushed to
  remote. Ex. `-nomad.sha=0b6b475e7da77fed25727ea9f01f155a58481b6c`
- `-nomad.version=string`: provision this version from
  [releases.hashicorp.com](https://releases.hashicorp.com/nomad). Ex. `-nomad.version=0.10.2`

Then deploy Nomad to the cluster by passing `-provision.terraform`
without a Nomad version flag:

```sh
NOMAD_E2E=1 go test -v .                   \
  -timeout 20m                             \
  -nomad.local_file=$(which nomad)         \
  -provision.terraform=./provisioning.json \
  -skipTests
```

- `-skipTests`: provisioning can take time, so it's best to skip tests

- `-timeout 20m`: depending on your cluster size and upload bandwidth the
  default 10m timeout may not be long enough for provisioning to finish 

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

If you want to run a specific suite, you can specify the `-suite` flag as
shown below. Only the suite with a matching `Framework.TestSuite.Component`
will be run, and all others will be skipped.

```sh
go test -v -suite=Consul .
```

If you want to run a specific test, you'll need to regex-escape some of the
test's name so that the test runner doesn't skip over framework struct method
names in the full name of the tests:

```sh
go test -v . -run 'TestE2E/Consul/\*consul\.ScriptChecksE2ETest/TestGroup'
                              ^       ^             ^               ^
                              |       |             |               |
                          Component   |             |           Test func    
                                      |             |
                                  Go Package      Struct
```

## I Want To...

### ...SSH Into One Of The Test Machines

You can use the Terraform output to find the IP address. The keys will
in the `./terraform/keys/` directory.

```sh
ssh -i keys/nomad-e2e-*.pem ubuntu@${EC2_IP_ADDR}
```

Run `terraform output` for IP addresses and details.

### ...Deploy a Cluster of Mixed Nomad Versions

The `provisioning.json` file output by Terraform has a blank field for
`nomad_sha` for each node of the cluster (server and client). You can
manually edit the file to replace this value with a `nomad_sha`,
`nomad_local_binary`, or `nomad_version` for each node to create a
cluster of mixed versions. The provisioning framework accepts any of
the following options for those fields:

- `nomad_sha`: This is a Nomad binary identified by its full commit
  SHA that's stored in a shared s3 bucket that Nomad team developers
  can access. That commit SHA can be from any branch that's pushed to
  remote.  (Ex.  `"nomad_sha":
  "0b6b475e7da77fed25727ea9f01f155a58481b6c"`)
- `nomad_local_binary`: This is a path to a Nomad binary on your own
  host.  (Ex. `"nomad_local_binary": "/home/me/nomad"`)
- `nomad_version`: This is a version number of Nomad that's been
  released to HashiCorp. (Ex. `"nomad_version": "0.10.2"`)

Then deploy Nomad to the cluster by passing `-provision.terraform`
without a Nomad version flag:

```sh
go test -v . -provision.terraform ./provisioning.json -skipTests
```

### ...Deploy Custom Configuration Files

The `provisioning.json` file includes a `bundles` section for each
node of the cluster (server and client). You can manually edit this
file to add, remove, or replace

```json
"bundles": [
  {
    "destination": "/ops/shared/nomad/base.hcl",
    "source": "/home/me/custom.hcl"
  }
]
```

### ...Deploy More Than 4 Linux Clients

Right now the framework doesn't support this out-of-the-box because of
the way the provisioning script adds specific client configurations to
each client node (for constraint testing). You'll need to add
additional configuration files to
`./e2e/terraform/shared/nomad/indexed`.
