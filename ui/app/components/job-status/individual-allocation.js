/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';

export default class JobStatusIndividualAllocationComponent extends Component {
  @alias('args.allocation.job.type') jobType;
  @alias('args.allocation.node.name') nodeName;
  @alias('args.allocation.taskGroup.name') groupName;
  @alias('args.allocation.job.taskGroups') taskGroups;
  @alias('args.allocation.shortId') shortId;

  get showClient() {
    return this.jobType === 'system' || this.jobType === 'sysbatch';
  }

  get tooltipText() {
    if (this.showClient) {
      return `${this.nodeName} - ${this.shortId}`;
    } else if (this.groupName && this.taskGroups?.length > 1) {
      return `${this.groupName} - ${this.shortId}`;
    } else if (this.shortId) {
      return this.shortId;
    } else {
      return 'oof';
    }
  }

  // If a user has hovered or otherwise focused a taskGroup,
  // and it differs from this alloc's task group, then fade it.
  get shouldFade() {
    return (
      this.groupName &&
      this.args.focusedGroup &&
      this.args.focusedGroup !== this.args.allocation.taskGroup
    );
  }
}
