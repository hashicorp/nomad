/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import { fn, array } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { and, eq } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import TaskContents from 'nomad-ui/components/exec/task-contents';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';

export default class TaskGroupParent extends Component {
  @service router;

  @tracked clickedOpen = false;

  get currentRouteIsThisTaskGroup() {
    const taskGroup = this.args.taskGroup;
    const route = this.router.currentRoute;

    if (!taskGroup || !route?.name?.includes('task-group')) {
      return false;
    }

    const taskGroupRoute = route.parent;
    const execRoute = taskGroupRoute?.parent;

    return (
      execRoute?.params?.job_name === taskGroup.job.name &&
      taskGroupRoute?.params?.task_group_name === taskGroup.name
    );
  }

  get isOpen() {
    return this.clickedOpen || this.currentRouteIsThisTaskGroup;
  }

  get hasPendingAllocations() {
    const allocations =
      this.args.taskGroup?.allocations?.toArray?.() ||
      this.args.taskGroup?.allocations;

    return (allocations || []).some(
      (allocation) => allocation.clientStatus === 'pending',
    );
  }

  get allocationTaskStates() {
    const allocations =
      this.args.taskGroup?.allocations?.toArray?.() ||
      this.args.taskGroup?.allocations;

    return (allocations || []).reduce((accumulator, allocation) => {
      const states =
        allocation?.states?.toArray?.() || allocation?.states || [];
      return accumulator.concat(states);
    }, []);
  }

  get activeTaskStates() {
    return this.allocationTaskStates.filter((taskState) => taskState?.isActive);
  }

  get tasksWithRunningStates() {
    const taskGroup = this.args.taskGroup;
    if (!taskGroup?.tasks) return [];

    const activeTaskStateNames = this.activeTaskStates
      .filter(
        (taskState) => taskState?.task?.taskGroup?.name === taskGroup.name,
      )
      .map((taskState) => taskState.name);

    return taskGroup.tasks.filter((task) =>
      activeTaskStateNames.includes(task.name),
    );
  }

  get sortedTasks() {
    return this.tasksWithRunningStates
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name));
  }

  toggleOpen = () => {
    this.clickedOpen = !this.clickedOpen;
  };

  openInNewWindow = (job, taskGroup, task) => {
    const url = generateExecUrl(this.router, {
      job,
      taskGroup,
      task,
    });

    openExecUrl(url);
  };

  <template>
    <button
      {{on "click" this.toggleOpen}}
      class="toggle-button {{if this.hasPendingAllocations 'is-loading'}}"
      data-test-task-group-name
      type="button"
    >
      <HdsIcon
        @name={{if this.isOpen "chevron-down" "chevron-right"}}
        @isInline={{true}}
        class="icon-vertical-bump-down"
      />
      {{@taskGroup.name}}
    </button>
    {{#if this.isOpen}}
      <ul class="task-list">
        {{#each this.sortedTasks as |task|}}
          {{#if @shouldOpenInNewWindow}}
            <a
              {{on
                "click"
                (fn this.openInNewWindow @taskGroup.job @taskGroup task)
              }}
              href="#"
              class="task-item"
              data-test-task
            >
              <TaskContents
                @task={{task}}
                @active={{and
                  this.currentRouteIsThisTaskGroup
                  (eq task.name @activeTaskName)
                }}
                @shouldOpenInNewWindow={{@shouldOpenInNewWindow}}
              />
            </a>
          {{else}}
            <LinkTo
              @route="exec.task-group.task"
              @models={{array @taskGroup.job.plainId @taskGroup.name task.name}}
              class="task-item"
              data-test-task={{true}}
            >
              <TaskContents
                @task={{task}}
                @active={{and
                  this.currentRouteIsThisTaskGroup
                  (eq task.name @activeTaskName)
                }}
                @shouldOpenInNewWindow={{@shouldOpenInNewWindow}}
              />
            </LinkTo>
          {{/if}}
        {{/each}}
      </ul>
    {{/if}}
  </template>
}
