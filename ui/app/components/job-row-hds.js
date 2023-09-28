/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';

export default class JobRowHdsComponent extends Component {
  @service router;
  @service store;
  @service system;

  @alias('args.job') job;

  // One of independent, parent, or child. Used to customize the template
  // based on the relationship of this job to others.
  @tracked jobLineage = 'independent';

  @action
  gotoJob() {
    this.router.transitionTo('jobs.job.index', this.job.idWithNamespace);
  }
}
