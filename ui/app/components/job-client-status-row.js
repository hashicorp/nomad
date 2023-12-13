/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import Component from '@glimmer/component';

export default class ClientRow extends Component {
  // Attribute set in the template as @onClick.
  onClick() {}

  get row() {
    return this.args.row.model;
  }

  get shouldDisplayAllocationSummary() {
    return this.args.row.model.jobStatus !== 'notScheduled';
  }

  get allocationSummaryPlaceholder() {
    switch (this.args.row.model.jobStatus) {
      case 'notScheduled':
        return 'Not Scheduled';
      default:
        return '';
    }
  }

  get humanizedJobStatus() {
    switch (this.args.row.model.jobStatus) {
      case 'notScheduled':
        return 'not scheduled';
      default:
        return this.args.row.model.jobStatus;
    }
  }

  get jobStatusClass() {
    switch (this.args.row.model.jobStatus) {
      case 'notScheduled':
        return 'not-scheduled';
      default:
        return this.args.row.model.jobStatus;
    }
  }

  get allocationContainer() {
    const statusSummary = {
      queuedAllocs: 0,
      completeAllocs: 0,
      failedAllocs: 0,
      runningAllocs: 0,
      startingAllocs: 0,
      lostAllocs: 0,
      unknownAllocs: 0,
    };

    switch (this.args.row.model.jobStatus) {
      case 'notSchedule':
        break;
      case 'queued':
        statusSummary.queuedAllocs = this.args.row.model.allocations.length;
        break;
      case 'starting':
        statusSummary.startingAllocs = this.args.row.model.allocations.length;
        break;
      default:
        for (const alloc of this.args.row.model.allocations) {
          switch (alloc.clientStatus) {
            case 'running':
              statusSummary.runningAllocs++;
              break;
            case 'lost':
              statusSummary.lostAllocs++;
              break;
            case 'failed':
              statusSummary.failedAllocs++;
              break;
            case 'complete':
              statusSummary.completeAllocs++;
              break;
            case 'starting':
              statusSummary.startingAllocs++;
              break;
            case 'unknown':
              statusSummary.unknownAllocs++;
              break;
          }
        }
    }

    const Allocations = EmberObject.extend({
      ...statusSummary,
    });
    return Allocations.create();
  }
}
