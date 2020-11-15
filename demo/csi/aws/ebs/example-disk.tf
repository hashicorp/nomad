data "aws_kms_alias" "ebs" {
  name = "alias/aws/ebs"
}

resource "aws_ebs_volume" "mytestebsvol" {
  availability_zone = "us-east-1a"
  size              = 1
 
  encrypted         = true
  kms_key_id        = data.aws_kms_alias.ebs.target_key_arn
    
  tags = {
    Name = "my-test-ebs-volume"
  }
}

output "mytestebsvol" {
    value = <<__EOF__
# terraform output mytestebsvol >example-volume.hcl
# nomad volume register example-folume.hcl

type = "csi"
id = "mytestebsvol"
name = "my-test-ebs-volume"
external_id = "${aws_ebs_volume.mytestebsvol.id}"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "aws-ebs"
__EOF__
}
