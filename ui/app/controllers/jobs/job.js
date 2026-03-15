/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { scheduleOnce } from '@ember/runloop';

export default class JobController extends Controller {
  @service router;
  @service notifications;
  @service store;
  _handlingNotFoundJob = false;
  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
  jobNamespace = 'default';

  get job() {
    return this.model;
  }

  @action async notFoundJobHandler() {
    if (this._handlingNotFoundJob) {
      return;
    }

    if (
      this.watchers.job.isError &&
      this.watchers.job.error?.errors?.some((e) => e.status === '404')
    ) {
      this._handlingNotFoundJob = true;
      scheduleOnce('actions', this, async () => {
        try {
          this.notifications.add({
            title: `Job "${this.job.name}" has been deleted`,
            message:
              'The job you were looking at has been deleted; this is usually because it was purged from elsewhere.',
            color: 'critical',
            sticky: true,
          });
          await this.router.transitionTo('jobs');
          this.store.unloadRecord(this.job);
        } catch (err) {
          if (err.code !== 'TRANSITION_ABORTED') {
            throw err;
          }
        } finally {
          this._handlingNotFoundJob = false;
        }
      });
    }
  }
}
