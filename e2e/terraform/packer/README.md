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

# build Ubuntu Bionic AMI
$ packer build ubuntu-bionic-amd64.pkr.hcl

# build Windows AMI
$ packer build windows-2016-amd64.pkr.hcl
```

## Debugging Packer Builds

You'll need the Windows administrator password in order to access Windows machines via `winrm` as Packer does. You can get this by enabling `-debug` on your Packer build.

```sh
packer build -debug -on-error=abort windows-2016-amd64.json
...
==> amazon-ebs: Pausing after run of step 'StepRunSourceInstance'. Press enter to continue.
==> amazon-ebs: Waiting for auto-generated password for instance...
    amazon-ebs: Password (since debug is enabled): <redacted>
```

Alternately, you can follow the steps in the [AWS documentation](https://aws.amazon.com/premiumsupport/knowledge-center/retrieve-windows-admin-password/). Note that you'll need the `ec2_amazon-ebs.pem` file that Packer drops in this directory.


Then in powershell (note the leading `$` here indicate variable declarations, not shell prompts!):

```
$username = "Administrator"
$password = "<redacted>"
$securePassword = ConvertTo-SecureString -AsPlainText -Force $password
$remoteHostname = "54.x.y.z"
$port = 5986
$cred = New-Object System.Management.Automation.PSCredential ($username, $securePassword)
$so = New-PSSessionOption -SkipCACheck -SkipCNCheck

Enter-PsSession `
    -ComputerName $remoteHostname `
    -Port $port `
    -Credential $cred `
    -UseSSL `
    -SessionOption $so `
    -Authentication Basic
```

Packer doesn't have a cleanup command if you've run `-on-error=abort`. So when you're done, clean up the machine by looking for "Packer" in the AWS console:
* [EC2 instances](https://console.aws.amazon.com/ec2/home?region=us-east-1#Instances:search=Packer;sort=tag:Name)
* [Key pairs](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#KeyPairs:search=packer;sort=keyName)
* [Security groups](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#SecurityGroups:search=packer;sort=groupName)
