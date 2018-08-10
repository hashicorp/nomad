region        = "us-east-1"
# ami           = "ami-ec237c93"
ami           = "ami-b53422ca"
instance_type = "t2.medium"
# key_name      = "KEY_NAME"
key_name      = "cv_hc-support-eng"
server_count  = "3"
client_count  = "4"

## Optional flags

# `name` allows the user to modify the name of infrastructure
# components created via this terraform script.  Defaults to
# "hashistack" if unset.
name          = "my_hashistack"

# `nomad_binary` will cause the nomad zip file at the given URL to be
# fetched and deoplyed over the top of the nomad execuatble included
# in the base AMI.
# nomad_binary  = "https://angrycub-hc.s3.amazonaws.com/public/linux_amd64.zip"
