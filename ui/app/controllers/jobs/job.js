/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobController extends Controller {
  @service router;
  @service notifications;
  @service store;
  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
  jobNamespace = 'default';

  get job() {
    return this.model;
  }

  @action notFoundJobHandler() {
    if (
      this.watchers.job.isError &&
      this.watchers.job.error?.errors?.some((e) => e.status === '404')
    ) {
      this.notifications.add({
        title: `Job "${this.job.name}" has been deleted`,
        message:
          'The job you were looking at has been deleted; this is usually because it was purged from elsewhere.',
        color: 'critical',
        sticky: true,
      });
      this.store.unloadRecord(this.job);
      this.router.transitionTo('jobs');
    }
  }
}
