/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import Component from '@glimmer/component';
import { service } from '@ember/service';
import { eq } from 'ember-truth-helpers';
import PromiseArray from 'nomad-ui/utils/classes/promise-array';
import AllocationRow from 'nomad-ui/components/allocation-row';
import ListTable from 'nomad-ui/components/list-table';
import TaskSubRow from 'nomad-ui/components/task-sub-row';
import Toggle from 'nomad-ui/components/toggle';
import pluralize from 'nomad-ui/helpers/pluralize';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class RecentAllocations extends Component {
  @service router;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @localStorageProperty('nomadShowSubTasks', true) showSubTasks;

  get sortedAllocations() {
    return PromiseArray.create({
      promise: this.args.job.allocations.then((allocations) =>
        allocations.sortBy('modifyIndex').reverse().slice(0, 5),
      ),
    });
  }

  toggleShowSubTasks = (event) => {
    event.preventDefault();
    this.showSubTasks = !this.showSubTasks;
  };

  gotoAllocation = (allocation) => {
    this.router.transitionTo('allocations.allocation', allocation.id);
  };

  <template>
    <div class="boxed-section" ...attributes>
      <div class="boxed-section-head">
        Recent Allocations
        <span class="pull-right is-padded">
          <Toggle
            @isActive={{this.showSubTasks}}
            @onToggle={{this.toggleShowSubTasks}}
            title="Show tasks of allocations"
          >Show Tasks</Toggle>
        </span>
      </div>
      <div
        class="boxed-section-body
          {{if @job.allocations.length 'is-full-bleed'}}"
      >
        {{#if @job.allocations.length}}
          <ListTable
            @source={{this.sortedAllocations}}
            @sortProperty={{this.sortProperty}}
            @sortDescending={{this.sortDescending}}
            @class="with-foot {{if this.showSubTasks 'with-collapsed-borders'}}"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health,
                  Scheduling, and Preemption</span></th>
              <th>
                ID
              </th>
              <th>
                Task Group
              </th>
              <th>
                Created
              </th>
              <th>
                Modified
              </th>
              <th>
                Status
              </th>
              <th>
                Version
              </th>
              <th>
                Client
              </th>
              <th>
                Volume
              </th>
              <th>
                CPU
              </th>
              <th>
                Memory
              </th>
              {{#if @job.actions.length}}
                <th>Actions</th>
              {{/if}}
            </t.head>
            <t.body as |row|>
              <AllocationRow
                data-test-allocation={{row.model.id}}
                @allocation={{row.model}}
                @context="job"
                @onClick={{fn this.gotoAllocation row.model}}
                @showSubTasks={{this.showSubTasks}}
                {{keyboardShortcutModifier
                  enumerated=true
                  action=(fn this.gotoAllocation row.model)
                }}
              />

              {{#if this.showSubTasks}}
                {{#each row.model.states as |task|}}
                  <TaskSubRow
                    @namespan="9"
                    @taskState={{task}}
                    @active={{eq
                      @activeTask
                      (concat task.allocation.id "-" task.name)
                    }}
                    @onSetActiveTask={{@setActiveTaskQueryParam}}
                    @jobHasActions={{@job.actions.length}}
                  />
                {{/each}}
              {{/if}}
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
      {{#if @job.allocations.length}}
        <div class="boxed-section-foot">
          <p class="pull-right" data-test-view-all-allocations>
            <LinkTo @route="jobs.job.allocations" @model={{@job}}>
              View all
              {{@job.allocations.length}}
              {{pluralize "allocation" @job.allocations.length}}
            </LinkTo>
          </p>
        </div>
      {{/if}}
    </div>
  </template>
}
