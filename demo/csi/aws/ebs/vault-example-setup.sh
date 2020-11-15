#!/bin/sh

# Vault AWS Secrets Engine

vault secrets enable -path=aws aws
vault write aws/config/root \
    access_key=A...NA \
    secret_key=123XYZ \
    region=us-east-1

# Permissions needed by the EBS CSI driver

vault write aws/roles/ebs-csi credential_type=iam_user \
   policy_arns="arn:aws:iam::481516234200:policy/EBS-CSI" \
   permissions_boundary_arn="arn:aws:iam::481516234200:policy/VaultGrantedBoundary"

# A policy to access AWS credentials for the driver
vault policy write ebs-csi ebs-csi.hcl
