/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { get } from '@ember/object';
import { task } from 'ember-concurrency';
import can from 'ember-can/helpers/can';
import { array } from '@ember/helper';
import { on } from '@ember/modifier';
import { didInsert } from '@ember/render-modifiers';
import {
  HdsAlert,
  HdsBadge,
  HdsButton,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import { and, eq, not } from 'ember-truth-helpers';
import { hash, concat } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import ConditionalLinkTo from 'nomad-ui/components/conditional-link-to';
import JobStatusAllocationStatusRow from 'nomad-ui/components/job-status/allocation-status-row';
import JobStatusDeploymentHistory from 'nomad-ui/components/job-status/deployment-history';
import JobStatusFailedOrLost from 'nomad-ui/components/job-status/failed-or-lost';
import JobStatusUpdateParams from 'nomad-ui/components/job-status/update-params';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { jobAllocStatuses } from 'nomad-ui/utils/allocation-client-statuses';

export default class JobStatusPanelDeploying extends Component {
  @tracked oldVersionAllocBlockIDs = [];

  get deployment() {
    return get(this.args.job, 'latestDeployment');
  }

  get deploymentVersionNumber() {
    return get(this.args.job, 'latestDeployment.versionNumber');
  }

  get allocTypes() {
    return jobAllocStatuses[this.args.job.type].map((type) => ({
      label: type,
    }));
  }

  get allocations() {
    const relationship = this.args.job?.hasMany?.('allocations');
    const ids = relationship?.ids?.() || [];
    const store = this.args.job?.store;

    if (!store || !ids.length) {
      return [];
    }

    return ids.map((id) => store.peekRecord('allocation', id)).filter(Boolean);
  }

  establishOldAllocBlockIDs = () => {
    this.oldVersionAllocBlockIDs = this.allocations.filter(
      (allocation) => allocation.clientStatus === 'running' && allocation.isOld,
    );
  };

  get canariesHealthy() {
    const relevantAllocs = this.allocations.filter(
      (allocation) =>
        !allocation.isOld &&
        allocation.isCanary &&
        !allocation.hasBeenRescheduled,
    );

    return (
      relevantAllocs.length &&
      relevantAllocs.every(
        (allocation) =>
          allocation.clientStatus === 'running' && allocation.isHealthy,
      )
    );
  }

  get someCanariesHaveFailed() {
    const relevantAllocs = this.allocations.filter(
      (allocation) =>
        !allocation.isOld &&
        allocation.isCanary &&
        !allocation.hasBeenRescheduled,
    );

    return relevantAllocs.some(
      (allocation) =>
        allocation.clientStatus === 'failed' ||
        allocation.clientStatus === 'lost' ||
        allocation.isUnhealthy,
    );
  }

  promote = task(async () => {
    try {
      await this.args.job.latestDeployment.content.promote();
    } catch (error) {
      this.args.handleError?.({
        title: 'Could Not Promote Deployment',
        description: messageFromAdapterError(error, 'promote deployments'),
      });
    }
  });

  fail = task(async () => {
    try {
      await this.args.job.latestDeployment.content.fail();
    } catch (error) {
      this.args.handleError?.({
        title: 'Could Not Fail Deployment',
        description: messageFromAdapterError(error, 'fail deployments'),
      });
    }
  });

  get desiredTotal() {
    return this.totalAllocs;
  }

  get oldVersionAllocBlocks() {
    return this.allocations
      .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
      .reduce((allocationGroups, currentAlloc) => {
        const status = currentAlloc.clientStatus;

        if (!allocationGroups[status]) {
          allocationGroups[status] = {
            healthy: { nonCanary: [] },
            unhealthy: { nonCanary: [] },
            health_unknown: { nonCanary: [] },
          };
        }

        allocationGroups[status].healthy.nonCanary.push(currentAlloc);
        return allocationGroups;
      }, {});
  }

  get newVersionAllocBlocks() {
    let availableSlotsToFill = this.desiredTotal;
    const allocationsOfDeploymentVersion = this.allocations.filter(
      (allocation) => !allocation.isOld,
    );

    const allocationCategories = this.allocTypes.reduce((categories, type) => {
      categories[type.label] = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
        health_unknown: { canary: [], nonCanary: [] },
      };
      return categories;
    }, {});

    for (const alloc of allocationsOfDeploymentVersion) {
      if (availableSlotsToFill <= 0) {
        break;
      }

      const status = alloc.clientStatus;
      const canary = alloc.isCanary ? 'canary' : 'nonCanary';
      const health =
        status === 'running'
          ? alloc.isHealthy
            ? 'healthy'
            : alloc.isUnhealthy
              ? 'unhealthy'
              : 'health_unknown'
          : 'health_unknown';

      if (allocationCategories[status]) {
        if (alloc.willNotRestart) {
          if (!alloc.willNotReschedule) {
            continue;
          }
        }

        allocationCategories[status][health][canary].push(alloc);
        availableSlotsToFill--;
      }
    }

    if (availableSlotsToFill > 0) {
      allocationCategories.unplaced = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
        health_unknown: { canary: [], nonCanary: [] },
      };
      allocationCategories.unplaced.healthy.nonCanary = Array(
        availableSlotsToFill,
      )
        .fill()
        .map(() => ({ clientStatus: 'unplaced' }));
    }

    return allocationCategories;
  }

  get newRunningHealthyAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks.running.healthy.canary,
      ...this.newVersionAllocBlocks.running.healthy.nonCanary,
    ];
  }

  get newRunningUnhealthyAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks.running.unhealthy.canary,
      ...this.newVersionAllocBlocks.running.unhealthy.nonCanary,
    ];
  }

  get newRunningHealthUnknownAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks.running.health_unknown.canary,
      ...this.newVersionAllocBlocks.running.health_unknown.nonCanary,
    ];
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

  get newAllocsByStatus() {
    return Object.entries(this.newVersionAllocBlocks).reduce(
      (counts, [status, healthStatusObj]) => {
        counts[status] = Object.values(healthStatusObj)
          .flatMap((canaryStatusObj) => Object.values(canaryStatusObj))
          .flatMap((canaryStatusArray) => canaryStatusArray).length;
        return counts;
      },
      {},
    );
  }

  get newAllocsByCanary() {
    const counts = Object.values(this.newVersionAllocBlocks)
      .flatMap((healthStatusObj) => Object.values(healthStatusObj))
      .flatMap((canaryStatusObj) => Object.entries(canaryStatusObj))
      .reduce((accumulator, [canaryStatus, items]) => {
        accumulator[canaryStatus] =
          (accumulator[canaryStatus] || 0) + items.length;
        return accumulator;
      }, {});

    return {
      canary: counts.canary || 0,
      nonCanary: counts.nonCanary || 0,
    };
  }

  get newAllocsByHealth() {
    return {
      healthy: this.newRunningHealthyAllocBlocks.length,
      unhealthy: this.newRunningUnhealthyAllocBlocks.length,
      health_unknown: this.newRunningHealthUnknownAllocBlocks.length,
    };
  }

  get oldRunningHealthyAllocBlocks() {
    return this.oldVersionAllocBlocks.running?.healthy?.nonCanary || [];
  }

  get oldCompleteHealthyAllocBlocks() {
    return this.oldVersionAllocBlocks.complete?.healthy?.nonCanary || [];
  }

  get totalAllocs() {
    return this.args.job.taskGroups.reduce(
      (sum, taskGroup) => sum + taskGroup.count,
      0,
    );
  }

  get deploymentIsAutoPromoted() {
    return get(this.args.job, 'latestDeployment.isAutoPromoted');
  }

  buildVersionEntries(versions) {
    return versions
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

  get oldVersions() {
    return this.buildVersionEntries(
      Object.values(this.oldRunningHealthyAllocBlocks),
    );
  }

  get newVersions() {
    const newVersionAllocs = Object.values(this.newVersionAllocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .filter(
        (allocation) =>
          allocation.jobVersion && allocation.jobVersion !== 'unknown',
      );

    return this.buildVersionEntries(newVersionAllocs);
  }

  get versions() {
    return [...this.oldVersions, ...this.newVersions];
  }

  get oldVersionLegend() {
    return [
      {
        count: this.oldRunningHealthyAllocBlocks.length,
        label: 'Running',
        status: 'running',
      },
      {
        count: this.oldCompleteHealthyAllocBlocks.length,
        label: 'Complete',
        status: 'complete',
      },
    ];
  }

  get newStatusLegend() {
    return Object.entries(this.newAllocsByStatus).map(([status, count]) => ({
      status,
      count,
      query: {
        status: `["${status}"]`,
        version: `[${this.deploymentVersionNumber}]`,
      },
      label: this.capitalize(status),
    }));
  }

  get newHealthLegend() {
    return Object.entries(this.newAllocsByHealth).map(([health, count]) => ({
      health,
      count,
      label: this.humanize(health),
    }));
  }

  capitalize(value) {
    if (!value) return '';
    return `${value.charAt(0).toUpperCase()}${value.slice(1)}`;
  }

  humanize(value) {
    if (!value) return '';
    return value
      .split('_')
      .map((word) => this.capitalize(word))
      .join(' ');
  }

  <template>
    <div
      class="job-status-panel boxed-section active-deployment"
      data-test-job-status-panel
    >
      <div class="boxed-section-head hds-foreground-primary">
        <div
          class="boxed-section-row"
          {{didInsert this.establishOldAllocBlockIDs}}
        >
          <h2>Status:
            <HdsBadge
              @text="Deploying {{@job.latestDeployment.shortId}}"
              @color="highlight"
              @type="filled"
            />
          </h2>
          <div class="pull-right">
            {{#if @job.latestDeployment.isRunning}}
              {{#if (can "fail deployment" namespace=@job.namespace)}}
                <HdsButton
                  data-test-fail
                  {{on "click" this.fail.perform}}
                  disabled={{this.fail.isRunning}}
                  @color="critical"
                  @text="Fail Deployment"
                  {{keyboardShortcutModifier
                    label="Fail Deployment"
                    pattern=(array "f" "a" "i" "l")
                    action=this.fail.perform
                  }}
                />
              {{/if}}
            {{/if}}
          </div>
        </div>
      </div>
      <div
        class="boxed-section-body
          {{if @job.latestDeployment.requiresPromotion 'requires-promotion'}}"
      >
        {{#if @job.latestDeployment.requiresPromotion}}
          <div class="canary-promotion-alert">
            {{#if this.canariesHealthy}}
              <HdsAlert @type="inline" @color="warning" as |A|>
                <A.Title>Deployment requires promotion</A.Title>
                <A.Description>Your deployment requires manual promotion — all
                  canary allocations have passed their health checks.</A.Description>
                {{#if (can "promote deployment" namespace=@job.namespace)}}
                  <A.Button
                    {{keyboardShortcutModifier
                      pattern=(array "p" "r" "o" "m" "o" "t" "e")
                      action=this.promote.perform
                    }}
                    data-test-promote-canary
                    @text="Promote Canary"
                    @color="primary"
                    {{on "click" this.promote.perform}}
                  />
                {{/if}}
              </HdsAlert>
            {{else if this.someCanariesHaveFailed}}
              <HdsAlert @type="inline" @color="critical" as |A|>
                <A.Title>Some Canaries have failed</A.Title>
                <A.Description>Your canary allocations have failed their health
                  checks. Please have a look at the error logs and task events
                  for the allocations in question.</A.Description>
              </HdsAlert>
            {{else}}
              <HdsAlert @type="inline" @color="neutral" as |A|>
                <A.Title>Checking Canary health</A.Title>
                {{#if this.deploymentIsAutoPromoted}}
                  <A.Description>Your canary allocations are being placed and
                    health-checked. If they pass, they will be automatically
                    promoted and your deployment will continue.</A.Description>
                {{else}}
                  <A.Description>Your job requires manual promotion, and your
                    canary allocations are being placed and health-checked.</A.Description>
                {{/if}}
              </HdsAlert>
            {{/if}}
          </div>
        {{/if}}

        <div class="deployment-allocations">
          {{#if this.oldVersionAllocBlockIDs.length}}
            <h4
              class="title is-5 previous-allocations-heading"
              data-test-old-allocation-tally
            >
              <span>
                Previous allocations:
                {{#if
                  this.oldVersionAllocBlocks.running
                }}{{this.oldRunningHealthyAllocBlocks.length}} running{{/if}}
              </span>

              <section class="versions">
                <ul>
                  {{#each this.oldVersions as |versionObj|}}
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
                            class="version-label"
                            @type="inverted"
                          />
                        {{else}}
                          <HdsBadge
                            @text="v{{versionObj.version}}"
                            class="version-label"
                            @type="inverted"
                          />
                        {{/if}}
                        <HdsBadge
                          @text={{versionObj.allocations.length}}
                          class="version-count"
                        />
                      </LinkTo>
                    </li>
                  {{/each}}
                </ul>
              </section>
            </h4>
            <div class="previous-allocations">
              <JobStatusAllocationStatusRow
                @allocBlocks={{this.oldVersionAllocBlocks}}
                @steady={{true}}
              />
            </div>
            <div
              class="legend-and-summary"
              data-test-previous-allocations-legend
            >
              <legend>
                {{#each this.oldVersionLegend as |item|}}
                  <span class="legend-item {{if (eq item.count 0) 'faded'}}">
                    <span class="represented-allocation {{item.status}}"></span>
                    <span class="count">{{item.count}} {{item.label}}</span>
                  </span>
                {{/each}}
              </legend>
            </div>

          {{/if}}

          <h4 class="title is-5" data-test-new-allocation-tally><span>New
              allocations:
              {{this.newRunningHealthyAllocBlocks.length}}/{{this.totalAllocs}}
              running and healthy</span>
            <span class="versions">
              <LinkTo
                data-version={{@job.version}}
                @route="jobs.job.allocations"
                @model={{@job}}
                @query={{hash version=(concat "[" @job.version "]")}}
              >
                <HdsBadge
                  @text="v{{@job.version}}"
                  @type="inverted"
                  class="version-label"
                />
              </LinkTo>
            </span>
          </h4>
          <div class="new-allocations">
            <JobStatusAllocationStatusRow
              @allocBlocks={{this.newVersionAllocBlocks}}
            />
          </div>
        </div>

        <div class="legend-and-summary" data-test-new-allocations-legend>
          <legend>
            {{#each this.newStatusLegend as |item|}}
              <ConditionalLinkTo
                @condition={{and (not (eq item.status "unplaced")) item.count}}
                @route="jobs.job.allocations"
                @model={{@job}}
                @query={{item.query}}
                @class="legend-item {{if (eq item.count 0) 'faded'}}"
                @label="View {{item.status}} allocations"
              >
                <span class="represented-allocation {{item.status}}"></span>
                <span class="count">{{item.count}} {{item.label}}</span>
              </ConditionalLinkTo>
            {{/each}}

            {{#each this.newHealthLegend as |item|}}
              <span class="legend-item {{if (eq item.count 0) 'faded'}}">
                <span
                  class="represented-allocation legend-example {{item.health}}"
                >
                  <span class="alloc-health-indicator">
                    {{#if (eq item.health "healthy")}}
                      <HdsIcon
                        @name="check"
                        @color="#25ba81"
                        @isInline={{true}}
                      />
                    {{else if (eq item.health "unhealthy")}}
                      <HdsIcon @name="x" @color="#c84034" @isInline={{true}} />
                    {{else}}
                      <HdsIcon
                        @name="running"
                        @color="black"
                        class="not-animated"
                        @isInline={{true}}
                      />
                    {{/if}}
                  </span>
                </span>
                <span class="count">{{item.count}} {{item.label}}</span>
              </span>
            {{/each}}

            <span
              class="legend-item
                {{if (eq this.newAllocsByCanary.canary 0) 'faded'}}"
            >
              <span class="represented-allocation legend-example canary">
                <span class="alloc-canary-indicator" />
              </span>
              <span class="count">{{this.newAllocsByCanary.canary}}
                Canary</span>
            </span>

          </legend>

          <JobStatusFailedOrLost
            @rescheduledAllocs={{this.rescheduledAllocs}}
            @restartedAllocs={{this.restartedAllocs}}
            @job={{@job}}
            @supportsRescheduling={{true}}
          />

        </div>

        <div class="history-and-params">
          <JobStatusDeploymentHistory @deployment={{@job.latestDeployment}} />
          <JobStatusUpdateParams @job={{@job}} />
        </div>

      </div>
    </div>
  </template>
}
