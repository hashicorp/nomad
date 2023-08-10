# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# `name` (required) is used to override the default decorator for elements in
# the stack.  This allows for more than one environment per account.
#  - This name can only contain alphanumeric characters.  If it is not provided 
#    here, it will be requested interactively.
name = "nomad"

# `key_name` (required) -  The name of the AWS SSH keys to be loaded on the
# instance at provisioning.  

# If it is not provided here, it will be requested interactively.
#key_name = "«YOUR EC2 SSH KEY NAME»"

# `nomad_binary` (optional, null) - URL of a zip file containing a nomad
# executable with which to replace the Nomad binaries in the AMI.
#  - Typically this is left commented unless necessary. 
#nomad_binary = "https://releases.hashicorp.com/nomad/0.10.0/nomad_0.10.0_linux_amd64.zip"

# `region` ("us-east-1") - sets the AWS region to build your cluster in.
#region = "us-east-1"

# `ami` (required) - The base AMI for the created nodes, This AMI must exist in
# the requested region for this environment to build properly.
#  - If it is not provided here, it will be requested interactively.
ami = "ami-09730698a875f6abd"

# `server_instance_type` ("t2.medium"), `client_instance_type` ("t2.medium"),
# `server_count` (3),`client_count` (4) - These options control instance size
# and count. They should be set according to your needs.
#
# * For the GPU demos, we used p3.2xlarge client instances.
# * For the Spark demos, you will need at least 4 t2.medium client
#   instances.
#server_instance_type = "t2.medium"
#server_count         = "3"
#client_instance_type = "t2.medium"
#client_count         = "4"

# `whitelist_ip` (required) - IP to whitelist for the security groups (set
# to 0.0.0.0/0 for world).  
#  - If it is not provided here, it will be requested interactively.
whitelist_ip = "0.0.0.0/0"
