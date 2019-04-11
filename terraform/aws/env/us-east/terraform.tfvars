region               = "us-east-1"
ami                  = "ami-090a41df9e193a506"
server_instance_type = "t2.medium"
client_instance_type = "t2.medium"
#for GPU work
#client_instance_type = "p3.2xlarge" // for
server_count         = "1"
client_count         = "1"
nomad_binary         = "https://releases.hashicorp.com/nomad/0.9.0/nomad_0.9.0_linux_amd64.zip"
