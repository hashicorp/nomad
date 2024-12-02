// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Find any released RPM or Deb in Artifactory. Requires the version, edition, distro, and distro
// version.
module "build_artifactory" {
  source = "./modules/fetch_artifactory"
}
