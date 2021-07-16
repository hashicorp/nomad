data "aws_iam_policy_document" "ebs_csi_policy_doc" {
  statement {
    effect  = "Allow"
    actions = [
       # Permissions needed by the EBS CSI driver

      "ec2:AttachVolume",
      "ec2:CreateSnapshot",
      "ec2:CreateTags",
      "ec2:CreateVolume",
      "ec2:DeleteSnapshot",
      "ec2:DeleteTags",
      "ec2:DeleteVolume",
      "ec2:DescribeAvailabilityZones",
      "ec2:DescribeInstances",
      "ec2:DescribeSnapshots",
      "ec2:DescribeTags",
      "ec2:DescribeVolumes",
      "ec2:DescribeVolumesModifications",
      "ec2:DetachVolume",
      "ec2:ModifyVolume"
    ]

    resources = [ "*" ]
  }
}

resource "aws_iam_policy" "ebs_csi_policy" {
  name        = "EBS-CSI"
  path        = "/"
  description = "Policy for EBS CSI Driver"

  policy = data.aws_iam_policy_document.ebs_csi_policy_doc.json
}

# Give the permissions to Vault so that it can give them to users
# it creates.

resource "aws_iam_group_policy_attachment" "vault_ebs_cli" {
  group      = aws_iam_group.vault_group.name
  policy_arn = aws_iam_policy.ebs_csi_policy.arn
}
