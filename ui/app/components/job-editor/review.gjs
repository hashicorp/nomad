/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';
import { and } from 'ember-truth-helpers';
import {
  HdsAlert,
  HdsButton,
  HdsButtonSet,
} from '@hashicorp/design-system-components/components';
import { htmlSafe } from '@ember/template';
import AllocationRow from 'nomad-ui/components/allocation-row';
import JobDiff from 'nomad-ui/components/job-diff';
import ListTable from 'nomad-ui/components/list-table';
import PlacementFailure from 'nomad-ui/components/placement-failure';

export default class JobEditorReview extends Component {
  // Slightly formats the warning string to be more readable
  get warnings() {
    return htmlSafe(
      (this.args.data.planOutput.warnings || '')
        .replace(/\n/g, '<br>')
        .replace(/\t/g, '&nbsp;&nbsp;&nbsp;&nbsp;'),
    );
  }

  run = () => {
    const submitTask = this.args.fns?.onSubmit;
    if (typeof submitTask?.perform === 'function') {
      submitTask.perform();
    }
  };

  <template>
    <div class="boxed-section">
      <div class="boxed-section-head">Job Plan</div>
      <div class="boxed-section-body is-dark">
        <JobDiff
          data-test-plan-output
          @diff={{@data.planOutput.diff}}
          @verbose={{false}}
        />
      </div>
    </div>

    <HdsAlert
      @type="inline"
      @color={{if @data.planOutput.failedTGAllocs "critical" "success"}}
      data-test-dry-run-message
      as |A|
    >
      <A.Title data-test-dry-run-title>Scheduler dry-run</A.Title>
      <A.Description data-test-dry-run-body>
        {{#if @data.planOutput.failedTGAllocs}}
          {{#each @data.planOutput.failedTGAllocs as |placementFailure|}}
            <PlacementFailure @failedTGAlloc={{placementFailure}} />
          {{/each}}
        {{else}}
          All tasks successfully allocated.
        {{/if}}
      </A.Description>
    </HdsAlert>
    <br />

    {{#if this.warnings}}
      <HdsAlert
        @type="inline"
        @color="warning"
        data-test-dry-run-warnings
        as |A|
      >
        <A.Description data-test-dry-run-warning-body>
          <p>
            {{this.warnings}}
          </p>
        </A.Description>
      </HdsAlert>
      <br />
    {{/if}}

    {{#if
      (and
        @data.planOutput.preemptions.isFulfilled
        @data.planOutput.preemptions.length
      )
    }}
      <div class="boxed-section is-warning" data-test-preemptions>
        <div class="boxed-section-head" data-test-preemptions-title>
          Preemptions (if you choose to run this job, these allocations will be
          stopped)
        </div>
        <div class="boxed-section-body" data-test-preemptions-body>
          <ListTable
            @source={{@data.planOutput.preemptions}}
            @class="allocations is-isolated"
            as |t|
          >
            <t.head>
              <th class="is-narrow">
                <span class="visually-hidden">
                  Driver Health, Scheduling, and Preemption
                </span>
              </th>
              <th>ID</th>
              <th>Task Group</th>
              <th>Created</th>
              <th>Modified</th>
              <th>Status</th>
              <th>Version</th>
              <th>Node</th>
              <th>Volume</th>
              <th>CPU</th>
              <th>Memory</th>
            </t.head>
            <t.body as |row|>
              <AllocationRow @allocation={{row.model}} @context="job" />
            </t.body>
          </ListTable>
        </div>
      </div>
    {{/if}}
    <HdsButtonSet class="is-associative">
      <HdsButton
        @text="Run"
        {{on "click" this.run}}
        disabled={{@fns.onSubmit.isRunning}}
        data-test-run
      />
      <HdsButton
        @color="secondary"
        @text="Cancel"
        {{on "click" @fns.onReset}}
        disabled={{@fns.onSubmit.isRunning}}
        data-test-cancel
      />
    </HdsButtonSet>
  </template>
}
