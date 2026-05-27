# Nomad Issue Labels

This document briefly describes the labels the Nomad team will apply when you
open a GitHub issue. The workflows described here are a work-in-progress.

### Types

Type labels define the workflow for an issue. See the description of the
workflows below.

Label | Description
---|---
type/enhancement | Proposed improvement or new feature
type/bug | Feature does not function as expected or crashes Nomad
type/question | General questions

### Stages

Triage labels define the stages of a workflow for an issue.

Label | Description
---|---
stage/accepted | The Nomad team intends to work on this bug or feature, but does not commit to a specific timeline. This doesn’t mean the design of the feature has been fully completed, just that we want to do so.
stage/thinking | The Nomad team member who triages the issue needs a few days to think and respond to the issue
stage/needs-discussion | This topic needs discussion with the larger Nomad maintainers group before committing to it. This doesn’t signify that design needs to be discussed.
stage/needs-investigation | The issue described is detailed and complex. It will need some work and can't be immediately resolved.
stage/waiting-reply | We need more information from the reporter.
stage/not-a-bug | Reported as a bug but turned out to be expected behavior and was closed.

### Themes

Theme labels define the component of Nomad involved. These will frequently
change and new themes will be added for new features, so see the description
of each label for details.

## Workflows

### `type/enhancement`

When you as a community member make a feature request, a Nomad maintainer will
triage it and generally label the issue as follows:

* `stage/thinking`: The Nomad team member who triages the issue wants to think
  about the idea some more.
* `stage/needs-discussion`: The Nomad team needs to discuss the idea within
  the larger maintainers group before committing to it.
* `stage/waiting-reply`: The Nomad maintainer needs you to provide some more
  information about the idea or its use cases.
* Closed: the Nomad team member may be able to tell right away that this
  request is not a good fit for Nomad.

The goal for issue labeled `stage/thinking`, `stage/needs-discussion`, or
`stage/waiting-reply` is to move them to `stage/accepted` (or to close
them). At this point, you can submit a PR that we'll be happy to review, the
Nomad maintainer who triaged the issue may open a PR, or for complex features
it will get into the Nomad team's roadmap for scheduling.

### `type/bug`

When you as a community member report a bug, a Nomad maintainer will triage it and generally label the issue as follows:

* `stage/needs-investigation`: The Nomad maintainer thinks this bug needs some
  initial investigation to determine if it's a bug or what system might be
  involved.
* `stage/waiting-reply`: The Nomad team member needs you to provide more
  information about the problem.
* `stage/accepted`: The bug will need more than a trivial amount of time to
  fix. Depending on the severity, the Nomad maintainers will work on fixing it
  immediately or get it into the roadmap for an upcoming release.
* `stage/not-a-bug`: The issue is not really a bug but is working as
  designed. Often this is a documentation issue, in which case the label may
  be changed to `type/enhancement` and `theme/docs`
* Fixed! If the issue is small, the Nomad maintainer may just immediately open
  a PR to fix the problem and will let you know to expect the in the next
  release.
