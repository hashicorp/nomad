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

  get showClient() {
    return this.jobType === 'system' || this.jobType === 'sysbatch';
  }
}
