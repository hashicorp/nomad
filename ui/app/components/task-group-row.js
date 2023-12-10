/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed, action } from '@ember/object';
import { alias, oneWay } from '@ember/object/computed';
import { debounce } from '@ember/runloop';
import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { lazyClick } from '../helpers/lazy-click';

@classic
@tagName('tr')
@classNames('task-group-row', 'is-interactive')
@attributeBindings('data-test-task-group')
export default class TaskGroupRow extends Component {
  @service can;

  taskGroup = null;
  debounce = 500;

  @oneWay('taskGroup.count') count;
  @alias('taskGroup.job.runningDeployment') runningDeployment;

  get namespace() {
    return this.get('taskGroup.job.namespace.name');
  }

  @computed('runningDeployment', 'namespace')
  get tooltipText() {
    if (this.can.cannot('scale job', null, { namespace: this.namespace }))
      return "You aren't allowed to scale task groups";
    if (this.runningDeployment)
      return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
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
      this.incrementProperty('count');
      this.scale(this.count);
    }
  }

  @action
  countDown() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.min == null || this.count > scaling.min) {
      this.decrementProperty('count');
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
