/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { lazyClick } from '../helpers/lazy-click';

export default class ChildJobRowComponent extends Component {
  @service router;

  click(event) {
    lazyClick([this.gotoJob, event]);
  }

  @action
  gotoJob() {
    const { job } = this.args;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }
}
