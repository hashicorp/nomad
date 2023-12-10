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
  @computed('taskState.state')
  get activeClass() {
    if (this.taskState && this.taskState.state === 'running') {
      return 'is-active';
    }

    return undefined;
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
