/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { tracked } from '@glimmer/tracking';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import { or } from 'ember-truth-helpers';
import AllocationRow from 'nomad-ui/components/allocation-row';
import AllocationStat from 'nomad-ui/components/allocation-stat';
import Tooltip from 'nomad-ui/components/tooltip';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import AllocationStatsTracker from 'nomad-ui/utils/classes/allocation-stats-tracker';

export default class PluginAllocationRow extends AllocationRow {
  @tracked allocation = null;

  buildStatsTracker(allocation) {
    if (!allocation?.isRunning) {
      return null;
    }

    return AllocationStatsTracker.create({
      fetch: (url) => this.token.authorizedRequest(url),
      allocation,
    });
  }

  syncPluginAllocation = () => {
    this.allocation = null;
    this.setAllocation();
  };

  updateStatsTracker = () => {
    this.statsTracker = this.buildStatsTracker(this.allocation);

    if (this.allocation) {
      this.qualifyAllocation();
    } else {
      this.fetchStats.cancelAll();
    }
  };

  qualifyAllocation = async () => {
    const allocation = this.allocation;
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
      if (job.isPartial) {
        await job.reload();
      }
    }

    this.fetchStats.perform();
  };

  setAllocation = async () => {
    if (this.args.pluginAllocation && !this.allocation) {
      const allocation = await this.args.pluginAllocation.getAllocation();
      if (!this.isDestroyed) {
        this.allocation = allocation;
        this.updateStatsTracker();
      }
    }
  };

  <template>
    <tr
      class="allocation-row"
      {{didInsert this.syncPluginAllocation}}
      {{didUpdate this.syncPluginAllocation @pluginAllocation}}
      ...attributes
    >
      {{#if this.allocation}}
        <td data-test-indicators class="is-narrow">
          {{#if this.allocation.unhealthyDrivers.length}}
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
          {{#if this.allocation.nextAllocation}}
            <span
              data-test-icon="reschedule"
              class="tooltip text-center"
              role="tooltip"
              aria-label="Allocation was rescheduled"
            >
              <HdsIcon
                @name="history"
                @color="faint"
                class="icon-vertical-bump-down"
              />
            </span>
          {{/if}}
          {{#if this.allocation.wasPreempted}}
            <span
              data-test-icon="preemption"
              class="tooltip text-center"
              role="tooltip"
              aria-label="Allocation was preempted"
            >
              <HdsIcon
                @name="cloud-lightning"
                @color="faint"
                class="icon-vertical-bump-down"
              />
            </span>
          {{/if}}
        </td>

        <td data-test-short-id>
          <LinkTo
            @route="allocations.allocation"
            @model={{this.allocation}}
            class="is-primary"
          >
            {{this.allocation.shortId}}
          </LinkTo>
        </td>

        <td data-test-create-time>
          <span
            class="tooltip"
            aria-label={{formatMonthTs this.allocation.createTime}}
          >
            {{formatMonthTs this.allocation.createTime short=true}}
          </span>
        </td>

        <td data-test-modify-time>
          <span
            class="tooltip"
            aria-label={{formatMonthTs this.allocation.modifyTime}}
          >
            {{momentFromNow this.allocation.modifyTime}}
          </span>
        </td>

        <td data-test-health>
          <span class="nowrap">
            <HdsIcon
              @name={{if
                @pluginAllocation.healthy
                "check-circle"
                "minus-circle"
              }}
              @color={{if @pluginAllocation.healthy "success" "critical"}}
              @isInline={{true}}
            />
            {{if @pluginAllocation.healthy "Healthy" "Unhealthy"}}
          </span>
        </td>

        <td data-test-client>
          <Tooltip @text={{this.allocation.node.name}}>
            <LinkTo @route="clients.client" @model={{this.allocation.node.id}}>
              {{this.allocation.node.shortId}}
            </LinkTo>
          </Tooltip>
        </td>
        <td>
          {{#if
            (or this.allocation.job.isPending this.allocation.job.isReloading)
          }}
            ...
          {{else}}
            <LinkTo
              @route="jobs.job"
              @model={{this.allocation.job}}
              data-test-job
            >{{this.allocation.job.name}}</LinkTo>
            <span class="is-faded" data-test-task-group>/
              {{this.allocation.taskGroup.name}}</span>
          {{/if}}
        </td>
        <td
          data-test-job-version
          class="is-1"
        >{{this.allocation.jobVersion}}</td>
        <td data-test-volume>{{if
            this.allocation.taskGroup.volumes.length
            "Yes"
          }}</td>

        <td data-test-cpu class="is-1 has-text-centered">
          <AllocationStat
            @metric="cpu"
            @allocation={{this.allocation}}
            @statsTracker={{this.stats}}
            @isLoading={{this.fetchStats.isRunning}}
            @error={{this.statsError}}
          />
        </td>
        <td data-test-mem class="is-1 has-text-centered">
          <AllocationStat
            @metric="memory"
            @allocation={{this.allocation}}
            @statsTracker={{this.stats}}
            @isLoading={{this.fetchStats.isRunning}}
            @error={{this.statsError}}
          />
        </td>
      {{else}}
        <td colspan="10">&hellip;</td>
      {{/if}}
    </tr>
  </template>
}
