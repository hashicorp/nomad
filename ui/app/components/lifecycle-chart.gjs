/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { get } from '@ember/object';
import { gt } from 'ember-truth-helpers';
import LifecycleChartRow from 'nomad-ui/components/lifecycle-chart-row';

export default class LifecycleChart extends Component {
  get lifecyclePhases() {
    const taskStates = normalizeCollection(this.args.taskStates);
    const tasks = normalizeCollection(this.args.tasks);
    const tasksOrStates = taskStates.length ? taskStates : tasks;
    const lifecycles = {
      'prestart-ephemerals': [],
      'prestart-sidecars': [],
      'poststart-ephemerals': [],
      'poststart-sidecars': [],
      poststops: [],
      mains: [],
    };

    tasksOrStates.forEach((taskOrState) => {
      const task = get(taskOrState, 'task') || taskOrState;

      const lifecycleName = get(task, 'lifecycleName');
      if (lifecycleName) {
        lifecycles[`${lifecycleName}s`].push(taskOrState);
      }
    });

    const phases = [];
    const stateActiveIterator = (state) => get(state, 'state') === 'running';

    if (lifecycles.mains.length < tasksOrStates.length) {
      phases.push({
        name: 'Prestart',
        cssClass: 'prestart',
        isActive: lifecycles['prestart-ephemerals'].some(stateActiveIterator),
      });

      phases.push({
        name: 'Main',
        cssClass: 'main',
        isActive:
          lifecycles.mains.some(stateActiveIterator) ||
          lifecycles['poststart-ephemerals'].some(stateActiveIterator),
      });

      phases.push({
        name: 'Poststart',
        cssClass: 'poststart',
      });

      phases.push({
        name: 'Poststop',
        cssClass: 'poststop',
        isActive: lifecycles.poststops.some(stateActiveIterator),
      });
    }

    return phases;
  }

  get sortedLifecycleTaskStates() {
    return normalizeCollection(this.args.taskStates).sort((a, b) => {
      return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
    });
  }

  get sortedLifecycleTasks() {
    return normalizeCollection(this.args.tasks).sort((a, b) => {
      return getTaskSortPrefix(a).localeCompare(getTaskSortPrefix(b));
    });
  }

  <template>
    {{#if (gt this.lifecyclePhases.length 1)}}
      <div class="boxed-section" data-test-lifecycle-chart ...attributes>
        <div class="boxed-section-head">
          Task Lifecycle
          {{if @taskStates "Status" "Configuration"}}
        </div>
        <div class="boxed-section-body lifecycle-chart">

          <div class="lifecycle-phases">
            {{#each this.lifecyclePhases as |phase|}}
              <div
                class="lifecycle-phase
                  {{if phase.isActive 'is-active'}}
                  {{phase.cssClass}}"
                data-test-lifecycle-phase
              >
                <div class="name" data-test-name>{{phase.name}}</div>
              </div>
            {{/each}}
            <svg class="divider prestart">
              <line x1="0" y1="0" x2="0" y2="100%" />
            </svg>
            <svg class="divider poststop">
              <line x1="0" y1="0" x2="0" y2="100%" />
            </svg>
          </div>

          <div class="lifecycle-chart-rows">
            {{#if @tasks}}
              {{#each this.sortedLifecycleTasks as |task|}}
                <LifecycleChartRow @task={{task}} />
              {{/each}}
            {{else}}
              {{#each this.sortedLifecycleTaskStates as |state|}}
                <LifecycleChartRow @taskState={{state}} @task={{state.task}} />
              {{/each}}
            {{/if}}
          </div>

        </div>
      </div>
    {{/if}}
  </template>
}

const lifecycleNameSortPrefix = {
  'prestart-ephemeral': 0,
  'prestart-sidecar': 1,
  main: 2,
  'poststart-sidecar': 3,
  'poststart-ephemeral': 4,
  poststop: 5,
};

function getTaskSortPrefix(task) {
  return `${lifecycleNameSortPrefix[task.lifecycleName]}-${task.name}`;
}

function normalizeCollection(value) {
  if (!value) {
    return [];
  }

  if (Array.isArray(value)) {
    return [...value];
  }

  if (typeof value.toArray === 'function') {
    return [...value];
  }

  if (typeof value[Symbol.iterator] === 'function') {
    return Array.from(value);
  }

  return [];
}
