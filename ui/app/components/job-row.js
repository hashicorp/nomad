/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { lazyClick } from '../helpers/lazy-click';
import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('job-row', 'is-interactive')
@attributeBindings('data-test-job-row')
export default class JobRow extends Component {
  @service router;
  @service store;
  @service system;

  job = null;

  // One of independent, parent, or child. Used to customize the template
  // based on the relationship of this job to others.
  context = 'independent';

  click(event) {
    lazyClick([this.gotoJob, event]);
  }

  @action
  gotoJob() {
    const { job } = this;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }
}
