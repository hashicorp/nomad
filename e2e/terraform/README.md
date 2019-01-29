Terraform provisioner for end to end tests
==========================================

This folder contains terraform resources for provisioning a nomad cluster on AWS for end to end tests.
It uses a Nomad binary identified by its commit SHA that's stored in a shared s3 bucket that Nomad team
developers can access. The commit SHA can be from any branch that's pushed to remote.

Use [envchain](https://github.com/sorah/envchain) to store your AWS credentials.


```
$ cd e2e/terraform/
$ envchain nomadaws TF_VAR_nomad_sha=<nomad_sha> terraform apply
```

After this step, you should have a nomad client address to point the end to end tests in the `e2e` folder to.

Teardown
========
The terraform state file stores all the info, so the nomad_sha doesn't need to be valid during teardown. 

```
$ cd e2e/terraform/
$ envchain nomadaws TF_VAR_nomad_sha=yyyzzz terraform destroy
```
