/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { get } from '@ember/object';
import { capitalize } from '@ember/string';
import { array } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { HdsAlert } from '@hashicorp/design-system-components/components';

const safeGet = (obj, key) => (obj ? get(obj, key) : undefined);

export default class LifecycleChartRow extends Component {
  get taskColor() {
    const state = safeGet(this.args.taskState, 'state');
    const failed = safeGet(this.args.taskState, 'failed');
    let color = 'neutral';
    if (state === 'running') {
      color = 'success';
    }
    if (state === 'pending') {
      color = 'neutral';
    }
    if (state === 'dead') {
      if (failed) {
        color = 'critical';
      } else {
        color = 'neutral';
      }
    }
    return color;
  }

  get taskIcon() {
    const state = safeGet(this.args.taskState, 'state');
    const failed = safeGet(this.args.taskState, 'failed');
    const startedAt = safeGet(this.args.taskState, 'startedAt');
    let icon;
    if (state === 'running') {
      icon = 'running';
    }
    if (state === 'pending') {
      icon = 'test';
    }
    if (state === 'dead') {
      if (failed) {
        icon = 'alert-circle';
      } else {
        if (startedAt) {
          icon = 'check-circle';
        } else {
          icon = 'minus-circle';
        }
      }
    }

    return icon;
  }

  get activeClass() {
    if (
      this.args.taskState &&
      get(this.args.taskState, 'state') === 'running'
    ) {
      return 'is-active';
    }

    return undefined;
  }

  get finishedClass() {
    if (this.args.taskState && get(this.args.taskState, 'state') === 'dead') {
      return 'is-finished';
    }

    return undefined;
  }

  get pendingClass() {
    return safeGet(this.args.taskState, 'state') === 'pending' ? 'pending' : '';
  }

  get lifecycleLabel() {
    if (!this.args.task) {
      return '';
    }

    const name = get(this.args.task, 'lifecycleName');

    if (name.includes('sidecar')) {
      return 'sidecar';
    } else if (name.includes('ephemeral')) {
      return name.substr(0, name.indexOf('-'));
    } else {
      return name;
    }
  }

  get lifecycleTitle() {
    return capitalize(this.lifecycleLabel);
  }

  <template>
    <div
      class="lifecycle-chart-row
        {{@task.lifecycleName}}
        {{this.activeClass}}
        {{this.finishedClass}}"
      data-test-lifecycle-task
      ...attributes
    >
      <HdsAlert
        @type="inline"
        @color={{this.taskColor}}
        class={{this.pendingClass}}
        @icon={{this.taskIcon}}
        as |A|
      >
        <A.Title class="name" data-test-name>
          {{#if @taskState}}
            <LinkTo
              @route="allocations.allocation.task"
              @models={{array @taskState.allocation @taskState.name}}
            >
              {{@task.name}}
            </LinkTo>
          {{else}}
            {{@task.name}}
          {{/if}}
        </A.Title>
        <A.Description>
          <div class="lifecycle" data-test-lifecycle>{{this.lifecycleTitle}}
            Task</div>
        </A.Description>
      </HdsAlert>
    </div>
  </template>
}
