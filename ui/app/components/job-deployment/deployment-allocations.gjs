/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import AllocationRow from 'nomad-ui/components/allocation-row';
import ListTable from 'nomad-ui/components/list-table';

export const JobDeploymentDeploymentAllocations = <template>
  <div data-test-deployment-allocations class="boxed-section">
    <div class="boxed-section-head">
      Allocations
    </div>
    <div
      class="boxed-section-body
        {{if @deployment.allocations.length 'is-full-bleed'}}"
    >
      {{#if @deployment.allocations.length}}
        <ListTable
          @source={{@deployment.allocations}}
          @class="allocations"
          as |t|
        >
          <t.head>
            <th class="is-narrow"><span class="visually-hidden">Driver Health,
                Scheduling, and Preemption</span></th>
            <th>ID</th>
            <th>Task Group</th>
            <th>Created</th>
            <th>Modified</th>
            {{#if @deployment.job.isBatchOrSysbatch}}
              <th>Max Run Deadline</th>
            {{/if}}
            <th>Status</th>
            <th>Version</th>
            <th>Node</th>
            <th>Volume</th>
            <th>CPU</th>
            <th>Memory</th>
          </t.head>
          <t.body as |row|>
            <AllocationRow
              data-test-deployment-allocation
              @allocation={{row.model}}
              @context="job"
              @showMaxRunDeadline={{@deployment.job.isBatchOrSysbatch}}
            />
          </t.body>
        </ListTable>
      {{else}}
        <div class="empty-message" data-test-empty-recent-allocations>
          <h3
            class="empty-message-headline"
            data-test-empty-recent-allocations-headline
          >
            No Allocations
          </h3>
          <p
            class="empty-message-body"
            data-test-empty-recent-allocations-message
          >
            No allocations have been placed.
          </p>
        </div>
      {{/if}}
    </div>
  </div>
</template>;

export default JobDeploymentDeploymentAllocations;
