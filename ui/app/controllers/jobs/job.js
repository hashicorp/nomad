/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobController extends Controller {
  @service router;
  @service notifications;
  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
  jobNamespace = 'default';

  get job() {
    return this.model;
  }

  get jobHasBeenYoinked() {
    return (
      this.watchers.job.isError &&
      this.watchers.job.error.errors.some((e) => e.status === '404')
    );
  }

  @action yoinkedJobHandler() {
    if (this.jobHasBeenYoinked) {
      this.notifications.add({
        title: `Job ${this.job.name} has been deleted`,
        message:
          'The job you were looking at has been deleted; this is usually because it was purged from elsewhere.',
        color: 'critical',
        destroyOnClick: false,
      });
      this.router.transitionTo('jobs');
    }
  }
}
