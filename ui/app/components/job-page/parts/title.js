/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@ember/component';
import { task } from 'ember-concurrency';
import { inject as service } from '@ember/service';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class Title extends Component {
  @service router;
  @service notifications;

  job = null;
  title = null;

  handleError() {}

  /**
   * @param {boolean} withNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNotifications = false) {
    try {
      const job = this.job;
      yield job.stop();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'dead');
      if (withNotifications) {
        this.notifications.add({
          title: 'Job Stopped',
          message: `${job.name} has been stopped`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Stop Job',
        description: messageFromAdapterError(err, 'stop jobs'),
      });
    }
  })
  stopJob;

  @task(function* () {
    try {
      const job = this.job;
      yield job.purge();
      this.notifications.add({
        title: 'Job Purged',
        message: `You have purged ${job.name}`,
        color: 'success',
      });
      this.router.transitionTo('jobs');
    } catch (err) {
      this.handleError({
        title: 'Error purging job',
        description: messageFromAdapterError(err, 'purge jobs'),
      });
    }
  })
  purgeJob;

  /**
   * @param {boolean} withNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNotifications = false) {
    const job = this.job;
    const specification = yield job.fetchRawSpecification();
    job.set('_newDefinition', specification.Source);

    try {
      yield job.parse();
      yield job.update();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'running');
      if (withNotifications) {
        this.notifications.add({
          title: 'Job Started',
          message: `${job.name} has started`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Start Job',
        description: messageFromAdapterError(err, 'start jobs'),
      });
    }
  })
  startJob;
}
