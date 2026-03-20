/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import momentFromNow from 'ember-moment/helpers/moment-from-now';

export default class ClientRow extends Component {
  // Attribute set in the template as @onClick.
  onClick() {}

  get row() {
    return this.args.row.model;
  }

  get allocation() {
    return this.args.allocation;
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

  <template>
    <tr
      data-test-client={{this.row.node.id}}
      class="job-client-status-row is-interactive"
      {{on "click" (fn @onClick this.row.node)}}
      ...attributes
    >
      <td data-test-short-id>
        <LinkTo
          @route="allocations.allocation"
          @model={{this.allocation}}
          class="is-primary"
        >
          {{this.row.node.shortId}}
        </LinkTo>
      </td>
      <td data-test-name class="is-200px is-truncatable">
        {{this.row.node.name}}
      </td>
      <td data-test-create-time>
        {{#if this.row.createTime}}
          <span
            class="tooltip"
            role="tooltip"
            aria-label={{formatMonthTs this.row.createTime}}
          >
            {{momentFromNow this.row.createTime}}
          </span>
        {{else}}
          -
        {{/if}}
      </td>
      <td data-test-modify-time>
        {{#if this.row.modifyTime}}
          <span
            class="tooltip"
            role="tooltip"
            aria-label={{formatMonthTs this.row.modifyTime}}
          >
            {{momentFromNow this.row.modifyTime}}
          </span>
        {{else}}
          -
        {{/if}}
      </td>
      <td data-test-job-status class="is-one-line">
        <span class="color-swatch {{this.jobStatusClass}}"></span>
        {{this.humanizedJobStatus}}
      </td>
      <td data-test-job-alloc-summary class="allocation-summary is-one-line">
        {{#if this.shouldDisplayAllocationSummary}}
          <div class="inline-chart">
            <AllocationStatusBar
              @allocationContainer={{this.allocationContainer}}
              @isNarrow={{true}}
            />
          </div>
        {{else}}
          <div class="is-faded">{{this.allocationSummaryPlaceholder}}</div>
        {{/if}}
      </td>
    </tr>
  </template>
}
