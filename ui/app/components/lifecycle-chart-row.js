/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class LifecycleChartRow extends Component {
  get taskColor() {
    let color;
    if (this.taskState?.state === 'running') {
      color = 'success';
    }
    if (this.taskState?.state === 'pending') {
      color = 'neutral';
    }
    if (this.taskState?.state === 'dead') {
      if (this.taskState?.failed) {
        color = 'critical';
      } else {
        color = 'neutral';
      }
    }
    return color;
  }

  get taskIcon() {
    let icon;
    if (this.taskState?.state === 'running') {
      icon = 'running';
    }
    if (this.taskState?.state === 'pending') {
      icon = 'test';
    }
    if (this.taskState?.state === 'dead') {
      if (this.taskState?.failed) {
        icon = 'alert-circle';
      } else {
        if (this.taskState?.startedAt) {
          icon = 'check-circle';
        } else {
          icon = 'minus-circle';
        }
      }
    }

    return icon;
  }

  @computed('taskState.state')
  get finishedClass() {
    if (this.taskState && this.taskState.state === 'dead') {
      return 'is-finished';
    }

    return undefined;
  }

  @computed('task.lifecycleName')
  get lifecycleLabel() {
    if (!this.task) {
      return '';
    }

    const name = this.task.lifecycleName;

    if (name.includes('sidecar')) {
      return 'sidecar';
    } else if (name.includes('ephemeral')) {
      return name.substr(0, name.indexOf('-'));
    } else {
      return name;
    }
  }
}
