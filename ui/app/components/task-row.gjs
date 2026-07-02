/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array, concat } from '@ember/helper';
import { tracked } from '@glimmer/tracking';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { and, not } from 'ember-truth-helpers';
import { task, timeout } from 'ember-concurrency';
import {
  HdsIcon,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';
import formatBytes from 'nomad-ui/helpers/format-bytes';
import formatHertz from 'nomad-ui/helpers/format-hertz';
import formatTs from 'nomad-ui/helpers/format-ts';
import formatVolumeName from 'nomad-ui/helpers/format-volume-name';
import ProxyTag from 'nomad-ui/components/proxy-tag';
import ENV from 'nomad-ui/config/environment';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class TaskRow extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

  @tracked statsError = false;

  get enablePolling() {
    return ENV.environment !== 'test';
  }

  get stats() {
    if (!this.args.task?.isRunning) return undefined;
    return this.statsTrackersRegistry.getTracker(this.args.task.allocation);
  }

  get taskStats() {
    if (!this.stats) return undefined;
    return this.stats.tasks?.find?.(
      (entry) => entry.task === this.args.task?.name,
    );
  }

  get cpu() {
    const cpu = this.taskStats?.cpu;
    return cpu?.[cpu.length - 1];
  }

  get memory() {
    const memory = this.taskStats?.memory;
    return memory?.[memory.length - 1];
  }

  click = (event) => {
    lazyClick([this.args.onClick, event]);
  };

  handleTaskChange = () => {
    const allocation = this.args.task?.allocation;

    if (allocation) {
      this.fetchStats.perform();
    } else {
      this.fetchStats.cancelAll();
    }
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
      class="task-row is-interactive"
      data-test-task-row
      {{on "click" this.click}}
      {{didInsert this.handleTaskChange}}
      {{didUpdate this.handleTaskChange @task.allocation}}
      ...attributes
    >
      <td class="is-narrow">
        {{#unless @task.driverStatus.healthy}}
          <HdsTooltipButton
            data-test-icon="unhealthy-driver"
            @text={{concat @task.driver " is unhealthy"}}
            aria-label="More information"
          >
            <HdsIcon @name="alert-triangle" @color="warning" />
          </HdsTooltipButton>
        {{/unless}}
      </td>
      <td data-test-name class="nowrap">
        <LinkTo
          @route="allocations.allocation.task"
          @models={{array @task.allocation @task.name}}
          class="is-primary"
        >
          {{@task.name}}
          {{#if @task.isConnectProxy}}
            <ProxyTag @class="bumper-left" />
          {{/if}}
        </LinkTo>
      </td>
      <td data-test-state>
        {{@task.state}}
      </td>
      <td data-test-message>
        {{#if @task.events.lastObject.message}}
          {{@task.events.lastObject.message}}
        {{else}}
          <em>
            No message
          </em>
        {{/if}}
      </td>
      <td data-test-time>
        {{formatTs @task.events.lastObject.time}}
      </td>
      <td data-test-volumes>
        <ul>
          {{#each @task.task.volumeMounts as |volume|}}
            <li data-test-volume>
              <strong>
                {{volume.volume}}
                :
              </strong>
              {{#if volume.isCSI}}
                <LinkTo
                  @route="storage.volumes.volume"
                  @model={{concat
                    (formatVolumeName
                      source=volume.source
                      isPerAlloc=volume.volumeDeclaration.perAlloc
                      volumeExtension=@task.allocation.volumeExtension
                    )
                    "@"
                    volume.namespace.id
                  }}
                >
                  {{formatVolumeName
                    source=volume.source
                    isPerAlloc=volume.volumeDeclaration.perAlloc
                    volumeExtension=@task.allocation.volumeExtension
                  }}
                </LinkTo>
              {{else}}
                {{volume.source}}
              {{/if}}
            </li>
          {{/each}}
        </ul>
      </td>
      <td data-test-cpu class="is-1 has-text-centered">
        {{#if @task.isRunning}}
          {{#if (and (not this.cpu) this.fetchStats.isRunning)}}
            ...
          {{else if this.statsError}}
            <span
              class="tooltip text-center"
              role="tooltip"
              aria-label="Couldn't collect stats"
            >
              <HdsIcon
                @name="alert-triangle-fill"
                @color="warning"
                class="icon-vertical-bump-down"
              />
            </span>
          {{else}}
            <div
              class="inline-chart is-small tooltip"
              role="tooltip"
              aria-label="{{formatHertz this.cpu.used}}
                 /
                {{formatHertz this.taskStats.reservedCPU}}"
            >
              <progress
                class="progress is-info is-small"
                value="{{this.cpu.percent}}"
                max="1"
              >
                {{this.cpu.percent}}
              </progress>
            </div>
          {{/if}}
        {{/if}}
      </td>
      <td data-test-mem class="is-1 has-text-centered">
        {{#if @task.isRunning}}
          {{#if (and (not this.memory) this.fetchStats.isRunning)}}
            ...
          {{else if this.statsError}}
            <span
              class="tooltip is-small text-center"
              role="tooltip"
              aria-label="Couldn't collect stats"
            >
              <HdsIcon
                @name="alert-triangle-fill"
                @color="warning"
                class="icon-vertical-bump-down"
              />
            </span>
          {{else}}
            <div
              class="inline-chart tooltip"
              role="tooltip"
              aria-label="{{formatBytes this.memory.used}}
                 /
                {{formatBytes this.taskStats.reservedMemory start="MiB"}}"
            >
              <progress
                class="progress is-danger is-small"
                value="{{this.memory.percent}}"
                max="1"
              >
                {{this.memory.percent}}
              </progress>
            </div>
          {{/if}}
        {{/if}}
      </td>
    </tr>
  </template>
}
