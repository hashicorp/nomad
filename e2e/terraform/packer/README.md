# Packer Builds

These builds are run as-needed to update the AMIs used by the end-to-end test infrastructure.


## What goes here?

* steps that aren't specific to a given Nomad build: ex. all Linux instances need `jq` and `awscli`.
* steps that aren't specific to a given EC2 instance: nothing that includes an IP address.
* steps that infrequently change: the version of Consul or Vault we ship.


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
