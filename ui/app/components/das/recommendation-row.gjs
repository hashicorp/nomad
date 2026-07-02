/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { didInsert } from '@ember/render-modifiers';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasRecommendationRow extends Component {
  @tracked cpu = {};
  @tracked memory = {};

  get taskGroup() {
    return this.args.summary.taskGroup;
  }

  get jobName() {
    return (
      this.taskGroup?.job?.name ||
      this.args.summary.job?.name ||
      this.args.summary.jobId
    );
  }

  get allocationCount() {
    return this.taskGroup?.count || 0;
  }

  storeDiffs = () => {
    // Prevent resource toggling from affecting the summary diffs
    if (!this.taskGroup) {
      this.cpu = {};
      this.memory = {};
      return;
    }

    const diffs = new ResourcesDiffs(
      this.taskGroup,
      1,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations,
    );

    const aggregateDiffs = new ResourcesDiffs(
      this.taskGroup,
      this.taskGroup.count,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations,
    );

    this.cpu = {
      delta: diffs.cpu.delta,
      signedDiff: diffs.cpu.signedDiff,
      percentDiff: diffs.cpu.percentDiff,
      signedAggregateDiff: aggregateDiffs.cpu.signedDiff,
    };

    this.memory = {
      delta: diffs.memory.delta,
      signedDiff: diffs.memory.signedDiff,
      percentDiff: diffs.memory.percentDiff,
      signedAggregateDiff: aggregateDiffs.memory.signedDiff,
    };
  };

  <template>
    {{#if @summary.taskGroup.allocations.length}}
      {{! Prevent storing aggregate diffs until allocation count is known }}
      <tr
        class="recommendation-row"
        ...attributes
        data-test-recommendation-summary-row
        {{didInsert this.storeDiffs}}
      >
        <td>
          <div data-test-slug>
            <span class="job">{{@summary.taskGroup.job.name}}</span>
            /
            <span class="task-group">{{@summary.taskGroup.name}}</span>
          </div>
          <div class="namespace">
            Namespace:
            <span data-test-namespace>{{@summary.jobNamespace}}</span>
          </div>
        </td>
        <td data-test-date>
          {{formatMonthTs @summary.submitTime}}
        </td>
        <td data-test-allocation-count>
          {{@summary.taskGroup.count}}
        </td>
        <td data-test-cpu>
          {{#if this.cpu.delta}}
            {{this.cpu.signedDiff}}
            <span class="percent">{{this.cpu.percentDiff}}</span>
          {{/if}}
        </td>
        <td data-test-memory>
          {{#if this.memory.delta}}
            {{this.memory.signedDiff}}
            <span class="percent">{{this.memory.percentDiff}}</span>
          {{/if}}
        </td>
        <td data-test-aggregate-cpu>
          {{#if this.cpu.delta}}
            {{this.cpu.signedAggregateDiff}}
          {{/if}}
        </td>
        <td data-test-aggregate-memory>
          {{#if this.memory.delta}}
            {{this.memory.signedAggregateDiff}}
          {{/if}}
        </td>
      </tr>
    {{/if}}
  </template>
}
