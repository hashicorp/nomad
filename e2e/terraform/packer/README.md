# Packer Builds

These builds are run as-needed to update the AMIs used by the end-to-end test
infrastructure.

## What goes here?

* steps that aren't specific to a given Nomad build: ex. all Linux instances
  need `jq` and `awscli`.
* steps that aren't specific to a given EC2 instance: nothing that includes an
  IP address.
* steps that infrequently change: the version of Consul or Vault we ship.

## How is this used?

The AMIs built by these Packer configs are tagged with `BuilderSha`, which has
the value of the most recent commit that touched this directory.

The nightly E2E job runs a script to see if there are any AMIs that match the
most recent commit that touched this directory, and if there aren't it will then
build the AMIs. Then most recent AMI with a matching SHA is used for the nightly
E2E run.

If you are changing this directory to build an AMI for testing, it's recommended
that you change the name of the AMI or make sure that you've locally committed
your changes so that your test AMI doesn't get picked up in the next nightly E2E
run.

## Running Packer builds

```sh
$ packer --version
1.6.4

# build Ubuntu Jammy AMI
$ ./build ubuntu-jammy-amd64

# build Windows AMI
$ ./build windows-2016-amd64
```

## Debugging Packer Builds

To [debug a Packer build](https://www.packer.io/docs/other/debugging.html)
you'll need to pass the `-debug` and `-on-error` flags. You can then ssh into
the instance using the `ec2_amazon-ebs.pem` file that Packer drops in this
directory.

Packer doesn't have a cleanup command if you've run `-on-error=abort`. So when
you're done, clean up the machine by looking for "Packer" in the AWS console:
* [EC2 instances](https://console.aws.amazon.com/ec2/home?region=us-east-1#Instances:search=Packer;sort=tag:Name)
* [Key pairs](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#KeyPairs:search=packer;sort=keyName)
* [Security groups](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#SecurityGroups:search=packer;sort=groupName)
