/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// import Component from '@ember/component';
import Component from '@glimmer/component';

import { inject as service } from '@ember/service';
import { computed, action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { debounce } from '@ember/runloop';
import { lazyClick } from '../helpers/lazy-click';

export default class TaskGroupRow extends Component {
  @service can;
  @service router;

  @alias('args.taskGroup') taskGroup;
  debounce = 500;

  @alias('taskGroup.count') count;
  @alias('taskGroup.job.runningDeployment') runningDeployment;

  @action
  gotoTaskGroup(taskGroup) {
    this.router.transitionTo(
      'jobs.job.task-group',
      taskGroup.job,
      taskGroup.name
    );
  }

  get namespace() {
    return this.taskGroup.job.namespace.get('name');
  }

  @computed('runningDeployment', 'namespace')
  get tooltipText() {
    if (this.can.cannot('scale job', null, { namespace: this.namespace }))
      return "You aren't allowed to scale task groups";
    if (this.runningDeployment)
      return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  click(event) {
    lazyClick([() => this.gotoTaskGroup(this.taskGroup), event]);
  }

  // TODO: DONT DO THIS, JUST PASS ON HOVER TO ALL JOB TYPES
  get onHover() {
    return this.args.onHover || (() => {});
  }

  @computed('count', 'taskGroup.scaling.min')
  get isMinimum() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.min == null) return false;
    return this.count <= scaling.min;
  }

  @computed('count', 'taskGroup.scaling.max')
  get isMaximum() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.max == null) return false;
    return this.count >= scaling.max;
  }

  @action
  countUp() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.max == null || this.count < scaling.max) {
      this.count++;
      this.scale(this.count);
    }
  }

  @action
  countDown() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.min == null || this.count > scaling.min) {
      this.count--;
      this.scale(this.count);
    }
  }

  scale(count) {
    debounce(this, sendCountAction, count, this.debounce);
  }
}

function sendCountAction(count) {
  return this.taskGroup.scale(count);
}
