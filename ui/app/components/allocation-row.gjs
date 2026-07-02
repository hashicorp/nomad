/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array } from '@ember/helper';
import { tracked } from '@glimmer/tracking';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { eq, notEq, or } from 'ember-truth-helpers';
import { task, timeout } from 'ember-concurrency';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import AllocationStat from 'nomad-ui/components/allocation-stat';
import Tooltip from 'nomad-ui/components/tooltip';
import formatJobId from 'nomad-ui/helpers/format-job-id';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import formatTs from 'nomad-ui/helpers/format-ts';
import ENV from 'nomad-ui/config/environment';
import AllocationStatsTracker from 'nomad-ui/utils/classes/allocation-stats-tracker';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class AllocationRow extends Component {
  @service store;
  @service token;

  @tracked statsError = false;
  @tracked statsTracker = null;

  get enablePolling() {
    if (typeof this.args.enablePolling === 'boolean') {
      return this.args.enablePolling;
    }

    return ENV.environment !== 'test';
  }

  get stats() {
    return this.statsTracker;
  }

  buildStatsTracker(allocation) {
    if (!allocation?.isRunning) {
      return null;
    }

    return AllocationStatsTracker.create({
      fetch: (url) => this.token.authorizedRequest(url),
      allocation,
    });
  }

  get cpu() {
    const cpu = this.stats?.cpu;
    return cpu?.[cpu.length - 1];
  }

  get memory() {
    const memory = this.stats?.memory;
    return memory?.[memory.length - 1];
  }

  get hasJobActions() {
    return Boolean(this.args.model?.job?.actions?.length);
  }

  click = (event) => {
    lazyClick([this.args.onClick, event]);
  };

  updateStatsTracker = () => {
    const allocation = this.args.allocation;
    this.statsTracker = this.buildStatsTracker(allocation);

    if (allocation) {
      this.qualifyAllocation();
    } else {
      this.fetchStats.cancelAll();
    }
  };

  qualifyAllocation = async () => {
    const allocation = this.args.allocation;
    if (!allocation) {
      return;
    }

    if (allocation.isPartial) {
      await this.store.findRecord('allocation', allocation.id, {
        backgroundReload: false,
      });
    }

    if (allocation.get('job.isPending')) {
      await allocation.get('job');
    } else if (!allocation.get('taskGroup')) {
      const job = allocation.get('job.content');
      if (job.isPartial) await job.reload();
    }

    this.fetchStats.perform();
  };

  fetchStats = task({ drop: true }, async () => {
    do {
      if (this.stats) {
        try {
          await this.stats.poll.linked().perform();
          this.statsError = false;
        } catch {
          this.statsError = true;
        }
      }

      await timeout(500);
    } while (this.enablePolling);
  });

  <template>
    <tr
      class="allocation-row is-interactive"
      {{on "click" this.click}}
      {{didInsert this.updateStatsTracker}}
      {{didUpdate this.updateStatsTracker @allocation}}
      ...attributes
    >
      <td data-test-indicators class="is-narrow">
        {{#if @allocation.unhealthyDrivers.length}}
          <span
            data-test-icon="unhealthy-driver"
            class="tooltip text-center"
            role="tooltip"
            aria-label="Allocation depends on unhealthy drivers"
          >
            <HdsIcon
              @name="alert-triangle-fill"
              @color="warning"
              class="icon-vertical-bump-down"
            />
          </span>
        {{/if}}
        {{#if @allocation.nextAllocation}}
          <span
            data-test-icon="reschedule"
            class="tooltip text-center"
            role="tooltip"
            aria-label="Allocation was rescheduled"
          >
            <HdsIcon @name="history" @color="faint" />
          </span>
        {{/if}}
        {{#if @allocation.wasPreempted}}
          <span
            data-test-icon="preemption"
            class="tooltip text-center"
            role="tooltip"
            aria-label="Allocation was preempted"
          >
            <HdsIcon @name="cloud-lightning" @color="faint" />
          </span>
        {{/if}}
      </td>
      <td data-test-short-id>
        <LinkTo
          @route="allocations.allocation"
          @model={{@allocation.id}}
          class="is-primary"
        >
          {{@allocation.shortId}}
        </LinkTo>
      </td>
      {{#if (eq @context "job")}}
        <td data-test-task-group>
          <LinkTo
            @route="jobs.job.task-group"
            @models={{array
              (formatJobId @allocation.job.id)
              @allocation.taskGroupName
            }}
          >
            {{@allocation.taskGroupName}}
          </LinkTo>
        </td>
      {{/if}}
      <td data-test-create-time>
        {{formatMonthTs @allocation.createTime}}
      </td>
      <td data-test-modify-time>
        <span
          class="tooltip"
          aria-label={{formatMonthTs @allocation.modifyTime}}
        >
          {{momentFromNow @allocation.modifyTime}}
        </span>
      </td>
      {{#if @showMaxRunDeadline}}
        <td data-test-max-run-deadline>
          {{#if @allocation.maxRunDeadline}}
            <span
              class="tooltip"
              aria-label={{formatTs @allocation.maxRunDeadline}}
            >
              {{momentFromNow @allocation.maxRunDeadline}}
            </span>
          {{/if}}
        </td>
      {{/if}}
      <td data-test-client-status class="is-one-line">
        <span class="color-swatch {{@allocation.clientStatus}}"></span>
        {{@allocation.clientStatus}}
      </td>
      {{#if (eq @context "volume")}}
        <td data-test-client>
          <Tooltip @text={{@allocation.node.name}}>
            <LinkTo @route="clients.client" @model={{@allocation.node.id}}>
              {{@allocation.node.shortId}}
            </LinkTo>
          </Tooltip>
        </td>
      {{/if}}
      {{#if (or (eq @context "taskGroup") (eq @context "job"))}}
        <td data-test-job-version>
          {{@allocation.jobVersion}}
        </td>
        <td data-test-client>
          <Tooltip @text={{@allocation.node.name}}>
            <LinkTo @route="clients.client" @model={{@allocation.node.id}}>
              {{@allocation.node.shortId}}
            </LinkTo>
          </Tooltip>
        </td>
      {{else if (or (eq @context "node") (eq @context "volume"))}}
        <td>
          {{#if (or @allocation.job.isPending @allocation.job.isReloading)}}
            ...
          {{else}}
            <LinkTo
              @route="jobs.job"
              @model={{formatJobId @allocation.job.id}}
              data-test-job
            >
              {{@allocation.job.name}}
            </LinkTo>
            <span class="is-faded" data-test-task-group>
              /
              {{@allocation.taskGroup.name}}
            </span>
          {{/if}}
        </td>
        <td data-test-job-version class="is-1">
          {{@allocation.jobVersion}}
        </td>
      {{/if}}
      {{#if (notEq @context "volume")}}
        <td data-test-volume>
          {{if @allocation.taskGroup.volumes.length "Yes"}}
        </td>
      {{/if}}
      <td data-test-cpu class="is-1 has-text-centered">
        <AllocationStat
          @metric="cpu"
          @allocation={{@allocation}}
          @statsTracker={{this.stats}}
          @isLoading={{this.fetchStats.isRunning}}
          @error={{this.statsError}}
        />
      </td>
      <td data-test-mem class="is-1 has-text-centered">
        <AllocationStat
          @metric="memory"
          @allocation={{@allocation}}
          @statsTracker={{this.stats}}
          @isLoading={{this.fetchStats.isRunning}}
          @error={{this.statsError}}
        />
      </td>
      {{#if this.hasJobActions}}
        <td class="job-actions-cell" />
      {{/if}}
    </tr>
  </template>
}
