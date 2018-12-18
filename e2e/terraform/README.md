Terraform provisioner for end to end tests
==========================================

This folder contains terraform resources for provisioning a nomad cluster on AWS for end to end tests.
It uses a nomad binary identified by its commit SHA that's stored in a shared s3 bucket that Nomad team
developers can access.

```
$ cd e2e/terraform/
$ TF_VAR_nomad_sha=<nomad_sha> terraform apply
```

After this step, you should have a nomad client address to point the end to end tests in the `e2e` folder to.