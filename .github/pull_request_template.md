### Description
<!-- Please describe why you're making this change and point out any important details the reviewers
should be aware of.-->

### Testing & Reproduction steps
<!--
* In the case of bugs, please describe how to reproduce it.
* If any manual tests were done, document the steps and the conditions to reproduce them.
-->

### Links
<!--
Please include links to GitHub issues, documentation, or similar which is relevant to this PR. If
this is a bug fix, please ensure related issues are linked so they will close when this PR is
merged.
-->

### Contributor Checklist
- [ ] **Changelog Entry** If this PR changes user-facing behavior, please generate and add a
  changelog entry using the `make cl` command.
- [ ] **Testing** Please add tests to cover any new functionality or to demonstrate bug fixes and
  ensure regressions will be caught.
- [ ] **Documentation** If the change impacts user-facing functionality such as the CLI, API, UI,
  and job configuration, please update the Nomad product documentation, which is stored in the
  [`web-unified-docs` repo](https://github.com/hashicorp/web-unified-docs/). Refer to the [`web-unified-docs` contributor guide](https://github.com/hashicorp/web-unified-docs/blob/main/CONTRIBUTING.md) for docs guidelines.
  Please also consider whether the change requires notes within the [upgrade
  guide](https://developer.hashicorp.com/nomad/docs/upgrade/upgrade-specific). If you would like help with the docs, tag the `nomad-docs` team in this PR.

### Reviewer Checklist
- [ ] **Backport Labels** Please add the correct backport labels as described by the internal
  backporting document.
- [ ] **Commit Type** Ensure the correct merge method is selected which should be "squash and merge"
  in the majority of situations. The main exceptions are long-lived feature branches or merges where
  history should be preserved.
- [ ] **Enterprise PRs** If this is an enterprise only PR, please add any required changelog entry
  within the public repository.


<!-- heimdall_github_prtemplate:grc-pci_dss-2024-01-05 -->

- [ ] If a change needs to be reverted, we will roll out an update to the code within 7 days.

## Changes to Security Controls

Are there any changes to security controls (access controls, encryption, logging) in this pull request? If so, explain.
