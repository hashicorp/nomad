/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { service } from '@ember/service';
import { action } from '@ember/object';
import { debounce, join } from '@ember/runloop';
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
  @service abilities;

  taskGroup = null;
  count = 0;
  debounce = 500;

  didReceiveAttrs() {
    super.didReceiveAttrs(...arguments);
    this.set('count', Number(this.taskGroup?.count ?? 0));
  }

  get runningDeployment() {
    return this.taskGroup?.job?.runningDeployment;
  }

  get namespace() {
    const job = this.taskGroup?.job;

    const namespaceId =
      (typeof job?.get === 'function' ? job.get('namespaceId') : undefined) ||
      job?.namespaceId;
    if (namespaceId) {
      return namespaceId;
    }

    const jobId =
      (typeof job?.get === 'function' ? job.get('id') : undefined) || job?.id;
    if (jobId) {
      try {
        const [, parsedNamespace] = JSON.parse(jobId);
        return parsedNamespace || 'default';
      } catch {
        // Fall through to final default.
      }
    }

    if (typeof job?.namespace === 'string') {
      return job.namespace;
    }

    return 'default';
  }

  get tooltipText() {
    if (this.abilities.cannot('scale job', null, { namespace: this.namespace }))
      return "You aren't allowed to scale task groups";
    if (this.runningDeployment)
      return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
  }

  get isMinimum() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.min == null) return false;
    return this.count <= scaling.min;
  }

  get isMaximum() {
    const scaling = this.taskGroup.scaling;
    if (!scaling || scaling.max == null) return false;
    return this.count >= scaling.max;
  }

  @action
  countUp() {
    join(this, () => {
      const scaling = this.taskGroup.scaling;
      if (!scaling || scaling.max == null || this.count < scaling.max) {
        const nextCount = this.count + 1;
        this.set('count', nextCount);
        this.taskGroup.set('count', nextCount);
        this.scale(nextCount);
      }
    });
  }

  @action
  countDown() {
    join(this, () => {
      const scaling = this.taskGroup.scaling;
      if (!scaling || scaling.min == null || this.count > scaling.min) {
        const nextCount = this.count - 1;
        this.set('count', nextCount);
        this.taskGroup.set('count', nextCount);
        this.scale(nextCount);
      }
    });
  }

  scale(count) {
    debounce(this, sendCountAction, count, this.debounce);
  }
}

function sendCountAction(count) {
  return this.taskGroup.scale(count);
}
