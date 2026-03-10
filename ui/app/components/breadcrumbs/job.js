/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assert } from '@ember/debug';
import { action } from '@ember/object';
import BreadcrumbsTemplate from './default';

export default class BreadcrumbsJob extends BreadcrumbsTemplate {
  get job() {
    return this.args.crumb.job;
  }

  get hasParent() {
    return !!this.job.belongsTo('parent').id();
  }

  @action
  traverseUpALevel() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }

  @action
  onError(err) {
    assert(`Error:  ${err.message}`);
  }

  @action
  fetchParent() {
    if (this.hasParent) {
      return this.job.get('parent');
    }
  }
}
