Terraform provisioner for end to end tests
==========================================

This folder contains terraform resources for provisioning a nomad cluster on AWS for end to end tests.
It uses a nomad binary identified by its commit SHA that's stored in a shared s3 bucket that Nomad team
developers can access.

Use [envchain](https://github.com/sorah/envchain) to store your AWS credentials.


```
$ cd e2e/terraform/
$ envchain nomadaws TF_VAR_nomad_sha=<nomad_sha> terraform apply
```

After this step, you should have a nomad client address to point the end to end tests in the `e2e` folder to.
