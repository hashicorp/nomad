/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { LinkTo } from '@ember/routing';
import { HdsBadge } from '@hashicorp/design-system-components/components';
import { and, eq, notEq } from 'ember-truth-helpers';
import JobPagePartsSummaryChart from 'nomad-ui/components/job-page/parts/summary-chart';
import ConditionalLinkTo from 'nomad-ui/components/conditional-link-to';
import JobStatusAllocationStatusRow from 'nomad-ui/components/job-status/allocation-status-row';
import JobStatusDeploymentHistory from 'nomad-ui/components/job-status/deployment-history';
import JobStatusFailedOrLost from 'nomad-ui/components/job-status/failed-or-lost';
import JobStatusLatestDeployment from 'nomad-ui/components/job-status/latest-deployment';
import { jobAllocStatuses } from 'nomad-ui/utils/allocation-client-statuses';
import { on } from '@ember/modifier';

export default class JobStatusPanelSteady extends Component {
  get allocations() {
    const relationship = this.args.job?.hasMany?.('allocations');
    const ids = relationship?.ids?.() || [];
    const store = this.args.job?.store;

    if (!store || !ids.length) {
      return [];
    }

    return ids.map((id) => store.peekRecord('allocation', id)).filter(Boolean);
  }

  get allocTypes() {
    return this.args.job.allocTypes;
  }

  get allocBlocks() {
    let availableSlotsToFill = this.totalAllocs;

    const allocationsOfShowableType = this.allocTypes.reduce(
      (accumulator, type) => {
        accumulator[type.label] = { healthy: { nonCanary: [] } };
        return accumulator;
      },
      {},
    );

    for (const alloc of this.allocations.filter(
      (allocation) =>
        allocation.clientStatus === 'running' ||
        allocation.clientStatus === 'pending',
    )) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
      availableSlotsToFill--;
    }

    const sortedAllocs = this.allocations
      .filter(
        (allocation) =>
          allocation.clientStatus !== 'running' &&
          allocation.clientStatus !== 'pending',
      )
      .sort((left, right) => {
        if (left.jobVersion > right.jobVersion) return 1;
        if (left.jobVersion < right.jobVersion) return -1;

        if (left.jobVersion === right.jobVersion) {
          return (
            jobAllocStatuses[this.args.job.type].indexOf(right.clientStatus) -
            jobAllocStatuses[this.args.job.type].indexOf(left.clientStatus)
          );
        }

        return 0;
      })
      .reverse();

    for (const alloc of sortedAllocs) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      if (
        this.allocTypes.map(({ label }) => label).includes(status) &&
        allocationsOfShowableType[status].healthy.nonCanary.length <
          this.totalAllocs
      ) {
        allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
        availableSlotsToFill--;
      }
    }

    if (availableSlotsToFill > 0) {
      allocationsOfShowableType.unplaced = {
        healthy: {
          nonCanary: Array(availableSlotsToFill)
            .fill()
            .map(() => ({ clientStatus: 'unplaced' })),
        },
      };
    }

    return allocationsOfShowableType;
  }

  get nodes() {
    return this.args.nodes;
  }

  get totalAllocs() {
    if (this.args.job.type === 'service' || this.args.job.type === 'batch') {
      return this.args.job.taskGroups.reduce(
        (sum, taskGroup) => sum + taskGroup.count,
        0,
      );
    } else if (this.atMostOneAllocPerNode) {
      return new Set(
        this.allocations
          .map((allocation) => allocation?.nodeID)
          .filter(Boolean),
      ).size;
    }

    return this.args.job.count;
  }

  get totalNonCompletedAllocs() {
    return this.totalAllocs - this.completedAllocs.length;
  }

  get allAllocsComplete() {
    return this.completedAllocs.length && this.totalNonCompletedAllocs === 0;
  }

  get atMostOneAllocPerNode() {
    return this.args.job.type === 'system' || this.args.job.type === 'sysbatch';
  }

  get versions() {
    return Object.values(this.allocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .map((allocation) =>
        !isNaN(allocation?.jobVersion) ? allocation.jobVersion : 'unknown',
      )
      .sort((left, right) => left - right)
      .reduce((result, item) => {
        const existingVersion = result.find(
          (version) => version.version === item,
        );
        if (existingVersion) {
          existingVersion.allocations.push(item);
        } else {
          result.push({
            version: item,
            allocations: [item],
            query: {
              version: `[${item}]`,
              status: '["running", "pending", "failed"]',
            },
          });
        }
        return result;
      }, []);
  }

  get versionsQueryString() {
    return `[${this.versions.map((version) => version.version).join(',')}]`;
  }

  get rescheduledAllocs() {
    return this.allocations.filter(
      (allocation) => !allocation.isOld && allocation.hasBeenRescheduled,
    );
  }

  get restartedAllocs() {
    return this.allocations.filter(
      (allocation) => !allocation.isOld && allocation.hasBeenRestarted,
    );
  }

  get runningAllocs() {
    return this.allocations.filter(
      (allocation) => allocation.clientStatus === 'running',
    );
  }

  get completedAllocs() {
    return this.allocations.filter(
      (allocation) =>
        !allocation.isOld && allocation.clientStatus === 'complete',
    );
  }

  get supportsRescheduling() {
    return this.args.job.type !== 'system';
  }

  get latestVersionAllocations() {
    return this.allocations.filter((allocation) => !allocation.isOld);
  }

  get currentStatus() {
    const totalAllocs = this.totalAllocs;

    if (this.args.job.status === 'dead' && this.args.job.stopped) {
      return { label: 'Stopped', state: 'neutral' };
    }

    if (this.totalAllocs === 0 && !this.args.job.hasClientStatus) {
      return { label: 'Scaled Down', state: 'neutral' };
    }

    if (this.args.job.type === 'batch' || this.args.job.type === 'sysbatch') {
      const completeAllocs = this.allocBlocks.complete?.healthy?.nonCanary;
      if (completeAllocs?.length === totalAllocs) {
        return { label: 'Complete', state: 'success' };
      }

      const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
      if (healthyAllocs?.length + completeAllocs?.length === totalAllocs) {
        return { label: 'Running', state: 'success' };
      }
    }

    const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
    if (healthyAllocs?.length && healthyAllocs?.length === totalAllocs) {
      return { label: 'Healthy', state: 'success' };
    }

    const pendingAllocs = this.allocBlocks.pending?.healthy?.nonCanary;
    if (pendingAllocs?.length > 0) {
      return { label: 'Recovering', state: 'highlight' };
    }

    const failedOrLostAllocs = [
      ...(this.allocBlocks.failed?.healthy?.nonCanary || []),
      ...(this.allocBlocks.lost?.healthy?.nonCanary || []),
      ...(this.allocBlocks.unplaced?.healthy?.nonCanary || []),
    ];

    if (failedOrLostAllocs.length === totalAllocs) {
      return { label: 'Failed', state: 'critical' };
    }

    return { label: 'Degraded', state: 'warning' };
  }

  get legendEntries() {
    return this.allocTypes.map((type) => {
      const count =
        this.allocBlocks[type.label]?.healthy?.nonCanary?.length || 0;
      return {
        type: type.label,
        count,
        label: this.capitalize(type.label),
        query: {
          status: `["${type.label}"]`,
          version: this.versionsQueryString,
        },
      };
    });
  }

  get runningSummary() {
    if (this.allAllocsComplete) {
      return 'All allocations have completed successfully';
    }

    const total = this.atMostOneAllocPerNode
      ? ''
      : this.args.job.type === 'batch'
        ? `/${this.totalNonCompletedAllocs}`
        : `/${this.totalAllocs}`;
    const remaining = this.args.job.type === 'batch' ? 'Remaining ' : '';
    const allocationLabel =
      this.runningAllocs.length === 1 ? 'Allocation' : 'Allocations';

    return `${this.runningAllocs.length}${total} ${remaining}${allocationLabel} Running`;
  }

  capitalize(value) {
    if (!value) return '';
    return `${value.charAt(0).toUpperCase()}${value.slice(1)}`;
  }

  setCurrentStatusMode = () => {
    this.args.setStatusMode?.('current');
  };

  setHistoricalStatusMode = () => {
    this.args.setStatusMode?.('historical');
  };

  <template>
    <div
      class="job-status-panel boxed-section steady-state
        {{if (eq @statusMode 'historical') 'historical-state' 'current-state'}}"
      data-test-job-status-panel
      data-test-status-mode={{@statusMode}}
    >
      <div class="boxed-section-head">
        <h2>Status:
          <HdsBadge
            @text={{this.currentStatus.label}}
            @color={{this.currentStatus.state}}
            @type="filled"
          /></h2>

        <div class="select-mode">
          <button
            type="button"
            data-test-status-mode-current
            class="button is-small is-borderless
              {{if (eq @statusMode 'current') 'is-active'}}"
            {{on "click" this.setCurrentStatusMode}}
          >
            Current
          </button>
          <button
            type="button"
            data-test-status-mode-historical
            class="button is-small is-borderless
              {{if (eq @statusMode 'historical') 'is-active'}}"
            {{on "click" this.setHistoricalStatusMode}}
          >
            Historical
          </button>
        </div>
      </div>
      <div class="boxed-section-body">
        {{#if (eq @statusMode "historical")}}
          <JobPagePartsSummaryChart @job={{@job}} />
        {{else}}
          <h3
            class="title is-4 running-allocs-title"
          >{{this.runningSummary}}</h3>
          <JobStatusAllocationStatusRow
            @allocBlocks={{this.allocBlocks}}
            @steady={{true}}
          />

          <div
            class="legend-and-summary
              {{if @job.latestDeployment 'has-latest-deployment'}}"
          >
            <legend>
              {{#each this.legendEntries as |item|}}
                <ConditionalLinkTo
                  @condition={{and (notEq item.type "unplaced") item.count}}
                  @route="jobs.job.allocations"
                  @model={{@job}}
                  @query={{item.query}}
                  @class="legend-item {{if (eq item.count 0) 'faded'}}"
                  @label="View {{item.type}} allocations"
                >
                  <span class="represented-allocation {{item.type}}"></span>
                  <span class="count">{{item.count}} {{item.label}}</span>
                </ConditionalLinkTo>
              {{/each}}
            </legend>

            <JobStatusFailedOrLost
              @rescheduledAllocs={{this.rescheduledAllocs}}
              @restartedAllocs={{this.restartedAllocs}}
              @job={{@job}}
              @supportsRescheduling={{this.supportsRescheduling}}
            />

            <section class="versions">
              <h4>Versions</h4>
              <ul>
                {{#each this.versions as |versionObj|}}
                  <li>
                    <LinkTo
                      data-version={{versionObj.version}}
                      @route="jobs.job.allocations"
                      @model={{@job}}
                      @query={{versionObj.query}}
                    >
                      {{#if (eq versionObj.version "unknown")}}
                        <HdsBadge
                          @text="unknown"
                          @type="inverted"
                          class="version-label"
                        />
                      {{else}}
                        <HdsBadge
                          @text="v{{versionObj.version}}"
                          @type="inverted"
                          class="version-label"
                        />
                      {{/if}}
                      <HdsBadge
                        @text={{versionObj.allocations.length}}
                        @type="filled"
                        class="version-count"
                      />
                    </LinkTo>
                  </li>
                {{/each}}
              </ul>
            </section>

            {{#if @job.latestDeployment}}
              <JobStatusLatestDeployment @job={{@job}} />
            {{/if}}

          </div>

          <div class="history-and-params">
            {{#if this.latestVersionAllocations.length}}
              <JobStatusDeploymentHistory
                @title="Allocation History"
                @allocations={{this.latestVersionAllocations}}
                @isHidden={{true}}
              />
            {{/if}}
          </div>

        {{/if}}
      </div>
    </div>
  </template>
}
