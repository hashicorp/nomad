/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import {
  HdsDropdown,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import can from 'ember-can/helpers/can';
import { task, timeout } from 'ember-concurrency';
import formatBytes from 'nomad-ui/helpers/format-bytes';
import formatHertz from 'nomad-ui/helpers/format-hertz';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import TaskContextSidebar from 'nomad-ui/components/task-context-sidebar';
import ENV from 'nomad-ui/config/environment';

export default class TaskSubRow extends Component {
  @service store;
  @service router;
  @service notifications;
  @service nomadActions;
  @service('stats-trackers-registry') statsTrackersRegistry;

  @tracked statsError = false;

  constructor() {
    super(...arguments);

    const allocation = this.task?.allocation;
    if (allocation) {
      this.fetchStats.perform();
    } else {
      this.fetchStats.cancelAll();
    }
  }

  get task() {
    return this.args.taskState;
  }

  get stats() {
    if (!this.task?.isRunning) return undefined;
    return this.statsTrackersRegistry.getTracker(this.task.allocation);
  }

  get enablePolling() {
    return ENV.environment !== 'test';
  }

  get taskStats() {
    if (!this.stats) return undefined;
    return this.stats.tasks.findBy('task', this.task.name);
  }

  get cpu() {
    const cpu = this.taskStats?.cpu;
    return cpu?.[cpu.length - 1];
  }

  get memory() {
    const memory = this.taskStats?.memory;
    return memory?.[memory.length - 1];
  }

  get isCpuLoading() {
    return !this.cpu && this.fetchStats.isRunning;
  }

  get isMemoryLoading() {
    return !this.memory && this.fetchStats.isRunning;
  }

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

  get shouldShowLogs() {
    return this.args.active;
  }

  get namespace() {
    return this.task.task?.taskGroup.job.namespace;
  }

  gotoTask = (allocation, task) => {
    const taskName =
      (typeof task?.get === 'function' ? task.get('name') : undefined) ||
      task?.name ||
      task;

    this.router.transitionTo(
      'allocations.allocation.task',
      allocation,
      taskName,
    );
  };

  handleTaskLogsClick = (task) => {
    this.args.onSetActiveTask?.(task);
  };

  closeSidebar = () => {
    this.args.onSetActiveTask?.(null);
  };

  get sidebarFns() {
    return {
      closeSidebar: this.closeSidebar,
    };
  }

  runAction = task(async (action, allocID) => {
    try {
      await this.nomadActions.runAction({ action, allocID });
    } catch (err) {
      this.notifications.add({
        title: `Error starting ${action.name}`,
        message: err,
        sticky: true,
        color: 'critical',
      });
    }
  });

  <template>
    <tr
      class="task-sub-row {{if @active 'is-active'}}"
      {{keyboardShortcut
        enumerated=true
        action=(fn this.gotoTask this.task.allocation this.task)
      }}
    >
      <td colspan={{@namespan}}>
        <div class="name-grid">
          <LinkTo
            title={{this.task.name}}
            class="task-name"
            @route="allocations.allocation.task"
            @models={{array this.task.allocation this.task}}
          >{{this.task.name}}</LinkTo>
          <button
            type="button"
            class="logs-sidebar-trigger button is-borderless is-inline is-compact"
            {{on "click" (fn this.handleTaskLogsClick this.task)}}
          >
            <HdsIcon @name="logs" @isInline={{true}} />View Logs
          </button>
        </div>
      </td>
      <td data-test-cpu class="is-1 has-text-centered">
        {{#if this.task.isRunning}}
          {{#if this.isCpuLoading}}
            ...
          {{else if this.statsError}}
            <span
              class="tooltip text-center"
              role="tooltip"
              aria-label="Couldn't collect stats"
            >
              <HdsIcon @name="alert-triangle" @color="warning" />
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
        {{#if this.task.isRunning}}
          {{#if this.isMemoryLoading}}
            ...
          {{else if this.statsError}}
            <span
              class="tooltip is-small text-center"
              role="tooltip"
              aria-label="Couldn't collect stats"
            >
              <HdsIcon @name="alert-triangle" @color="warning" />
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
      {{#if @jobHasActions}}
        <td data-test-actions>
          {{#if (can "exec allocation" namespace=this.namespace)}}
            {{#if this.task.task.actions.length}}
              <HdsDropdown class="actions-dropdown" as |dd|>
                <dd.ToggleIcon
                  @size="small"
                  @icon="more-horizontal"
                  @text="Actions"
                  @hasChevron={{false}}
                />
                {{#each this.task.task.actions as |actionC|}}
                  <dd.Interactive
                    data-test-task-row-action
                    {{on
                      "click"
                      (fn
                        this.runAction.perform actionC this.task.allocation.id
                      )
                    }}
                    @text={{actionC.name}}
                  />
                {{/each}}
              </HdsDropdown>
            {{/if}}
          {{/if}}
        </td>
      {{/if}}
    </tr>

    {{yield}}

    {{#if this.shouldShowLogs}}
      <TaskContextSidebar @task={{this.task}} @fns={{this.sidebarFns}} />
    {{/if}}
  </template>
}
