/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { capitalize } from '@ember/string';
import { array } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { eq } from 'ember-truth-helpers';
import { HdsAlert } from '@hashicorp/design-system-components/components';

export default class LifecycleChartRow extends Component {
  get taskColor() {
    let color = 'neutral';
    if (this.args.taskState?.state === 'running') {
      color = 'success';
    }
    if (this.args.taskState?.state === 'pending') {
      color = 'neutral';
    }
    if (this.args.taskState?.state === 'dead') {
      if (this.args.taskState?.failed) {
        color = 'critical';
      } else {
        color = 'neutral';
      }
    }
    return color;
  }

  get taskIcon() {
    let icon;
    if (this.args.taskState?.state === 'running') {
      icon = 'running';
    }
    if (this.args.taskState?.state === 'pending') {
      icon = 'test';
    }
    if (this.args.taskState?.state === 'dead') {
      if (this.args.taskState?.failed) {
        icon = 'alert-circle';
      } else {
        if (this.args.taskState?.startedAt) {
          icon = 'check-circle';
        } else {
          icon = 'minus-circle';
        }
      }
    }

    return icon;
  }

  get activeClass() {
    if (this.args.taskState && this.args.taskState.state === 'running') {
      return 'is-active';
    }

    return undefined;
  }

  get finishedClass() {
    if (this.args.taskState && this.args.taskState.state === 'dead') {
      return 'is-finished';
    }

    return undefined;
  }

  get lifecycleLabel() {
    if (!this.args.task) {
      return '';
    }

    const name = this.args.task.lifecycleName;

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
    >
      <HdsAlert
        @type="inline"
        @color={{this.taskColor}}
        class="{{if (eq @taskState.state 'pending') 'pending'}}"
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
